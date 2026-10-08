package my

import (
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecProfile permite ao próprio usuário trocar a foto de perfil, que o app RustDesk exibe
// (por exemplo, na janela de permissão de uma sessão).
type NextecProfile struct {
}

type nextecAvatarForm struct {
	// imagem em data URL (data:image/png;base64,...); vazio remove a foto
	Avatar string `json:"avatar"`
}

const nextecAvatarMaxBytes = 150 * 1024

var nextecAvatarPrefixes = []string{"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/webp;base64,"}

// Avatar grava ou remove a foto do usuário logado
// @Tags 我的
// @Summary Foto de perfil
// @Accept  json
// @Produce  json
// @Param body body nextecAvatarForm true "avatar"
// @Success 200 {object} response.Response
// @Router /admin/my/profile/avatar [post]
// @Security token
func (ct *NextecProfile) Avatar(c *gin.Context) {
	f := &nextecAvatarForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if f.Avatar != "" {
		ok := false
		for _, p := range nextecAvatarPrefixes {
			if strings.HasPrefix(f.Avatar, p) {
				raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(f.Avatar, p))
				ok = err == nil && len(raw) > 0 && len(raw) <= nextecAvatarMaxBytes
				break
			}
		}
		if !ok {
			response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
			return
		}
	}
	u := service.AllService.UserService.CurUser(c)
	upd := &model.User{}
	upd.Id = u.Id
	upd.Avatar = f.Avatar
	if err := service.UpdateFields(upd, "avatar"); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, gin.H{"avatar": f.Avatar})
}
