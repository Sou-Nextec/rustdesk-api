package my

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/model/custom_types"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"gorm.io/gorm"
)

// NextecShared expõe no painel o que o app RustDesk já mostra ao usuário:
// as listas compartilhadas com ele e a situação online dos dispositivos que ele enxerga.
type NextecShared struct {
}

type nextecSharedCollection struct {
	Id      uint   `json:"id"`
	Name    string `json:"name"`
	Owner   string `json:"owner"`
	OwnerId uint   `json:"owner_id"`
	Rule    int    `json:"rule"`
	Total   int64  `json:"total"`
}

type nextecSharedEntry struct {
	RowId          uint                  `json:"row_id"`
	Id             string                `json:"id"`
	Username       string                `json:"username"`
	Hostname       string                `json:"hostname"`
	Alias          string                `json:"alias"`
	Platform       string                `json:"platform"`
	Tags           custom_types.AutoJson `json:"tags" swaggertype:"array,string"`
	CollectionId   uint                  `json:"collection_id"`
	LastOnlineTime int64                 `json:"last_online_time"`
}

type nextecStatusForm struct {
	Ids []string `json:"ids" binding:"required"`
}

// readableCollections devolve as listas compartilhadas com o usuário (por pessoa ou equipe) e o maior nível de cada uma
func readableCollections(u *model.User) (map[uint]int, []*model.AddressBookCollection) {
	rules := map[uint]int{}
	for _, r := range service.AllService.AddressBookService.CollectionReadRules(u) {
		if rules[r.CollectionId] < r.Rule {
			rules[r.CollectionId] = r.Rule
		}
	}
	ids := make([]uint, 0, len(rules))
	for id := range rules {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return rules, nil
	}
	return rules, service.AllService.AddressBookService.ListCollectionByIds(ids)
}

// lastOnline busca a última comunicação de cada ID no cadastro de dispositivos
func lastOnline(ids []string) map[string]int64 {
	res := map[string]int64{}
	if len(ids) == 0 {
		return res
	}
	pl := service.AllService.PeerService.List(1, 99999, func(tx *gorm.DB) {
		tx.Select("id,last_online_time")
		tx.Where("id in (?)", ids)
	})
	for _, p := range pl.Peers {
		res[p.Id] = p.LastOnlineTime
	}
	return res
}

// Collections lista as listas compartilhadas com o usuário logado
// @Tags 我的
// @Summary Listas compartilhadas comigo
// @Produce  json
// @Success 200 {object} response.Response
// @Router /admin/my/shared/collections [get]
// @Security token
func (ct *NextecShared) Collections(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	rules, cols := readableCollections(u)
	owners := map[uint]string{}
	ownerIds := []uint{}
	for _, col := range cols {
		ownerIds = append(ownerIds, col.UserId)
	}
	for _, o := range service.AllService.UserService.ListByIds(ownerIds) {
		owners[o.Id] = o.Username
	}
	list := make([]*nextecSharedCollection, 0, len(cols))
	for _, col := range cols {
		if col.UserId == u.Id {
			continue // as próprias listas já aparecem em Minhas listas
		}
		total := service.AllService.AddressBookService.ListByUserIdAndCollectionId(col.UserId, col.Id, 1, 1).Total
		list = append(list, &nextecSharedCollection{
			Id: col.Id, Name: col.Name, Owner: owners[col.UserId], OwnerId: col.UserId, Rule: rules[col.Id], Total: total,
		})
	}
	response.Success(c, gin.H{"list": list, "total": len(list)})
}

// AddressBooks lista os dispositivos de uma lista compartilhada (sem senha salva)
// @Tags 我的
// @Summary Dispositivos de uma lista compartilhada comigo
// @Produce  json
// @Param collection_id query int true "lista"
// @Success 200 {object} response.Response
// @Router /admin/my/shared/address_book/list [get]
// @Security token
func (ct *NextecShared) AddressBooks(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	cid, _ := strconv.Atoi(c.Query("collection_id"))
	rules, cols := readableCollections(u)
	var target []*model.AddressBookCollection
	for _, col := range cols {
		if col.UserId == u.Id {
			continue
		}
		if cid == 0 || col.Id == uint(cid) {
			target = append(target, col)
		}
	}
	if cid != 0 && len(target) == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "NoAccess"))
		return
	}
	list := []*nextecSharedEntry{}
	ids := []string{}
	for _, col := range target {
		if !service.AllService.AddressBookService.CheckUserReadPrivilege(u, col.UserId, col.Id) || rules[col.Id] == 0 {
			continue
		}
		al := service.AllService.AddressBookService.ListByUserIdAndCollectionId(col.UserId, col.Id, 1, 10000)
		for _, a := range al.AddressBooks {
			list = append(list, &nextecSharedEntry{
				RowId: a.RowId, Id: a.Id, Username: a.Username, Hostname: a.Hostname, Alias: a.Alias,
				Platform: a.Platform, Tags: a.Tags, CollectionId: a.CollectionId,
			})
			ids = append(ids, a.Id)
		}
	}
	online := lastOnline(ids)
	for _, e := range list {
		e.LastOnlineTime = online[e.Id]
	}
	response.Success(c, gin.H{"list": list, "total": len(list)})
}

// Status devolve a última comunicação dos IDs que o usuário enxerga
// (nas próprias listas, nas listas compartilhadas com ele ou nos próprios dispositivos)
// @Tags 我的
// @Summary Situação online dos meus acessos
// @Accept  json
// @Produce  json
// @Param body body nextecStatusForm true "ids"
// @Success 200 {object} response.Response
// @Router /admin/my/shared/status [post]
// @Security token
func (ct *NextecShared) Status(c *gin.Context) {
	f := &nextecStatusForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	visible := map[string]bool{}
	for _, a := range service.AllService.AddressBookService.ListByUserId(u.Id, 1, 100000).AddressBooks {
		visible[a.Id] = true
	}
	_, cols := readableCollections(u)
	for _, col := range cols {
		if col.UserId == u.Id || !service.AllService.AddressBookService.CheckUserReadPrivilege(u, col.UserId, col.Id) {
			continue
		}
		for _, a := range service.AllService.AddressBookService.ListByUserIdAndCollectionId(col.UserId, col.Id, 1, 100000).AddressBooks {
			visible[a.Id] = true
		}
	}
	mine := service.AllService.PeerService.List(1, 100000, func(tx *gorm.DB) {
		tx.Select("id")
		tx.Where("user_id = ?", u.Id)
	})
	for _, p := range mine.Peers {
		visible[p.Id] = true
	}
	ids := []string{}
	for _, id := range f.Ids {
		if visible[id] {
			ids = append(ids, id)
		}
	}
	online := lastOnline(ids)
	list := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		list = append(list, gin.H{"id": id, "last_online_time": online[id]})
	}
	response.Success(c, gin.H{"list": list})
}
