package service

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Políticas do app e conexões ativas (Nextec).
//
// O app RustDesk (1.4.x) envia a cada poucos segundos um heartbeat ao servidor da API com os IDs das conexões abertas
// (`conns`) e o carimbo da última política recebida (`modified_at`). A resposta pode trazer `strategy.config_options`
// (opções a aplicar) e `disconnect` (IDs de conexão a derrubar). Sem política cadastrada nada é enviado.

// NextecPolicyOptions é a lista fechada de opções que o painel gerencia, com os valores aceitos.
var NextecPolicyOptions = map[string][]string{
	"enable-keyboard":       {"Y", "N"},
	"enable-clipboard":      {"Y", "N"},
	"enable-file-transfer":  {"Y", "N"},
	"enable-audio":          {"Y", "N"},
	"enable-camera":         {"Y", "N"},
	"enable-terminal":       {"Y", "N"},
	"enable-tunnel":         {"Y", "N"},
	"enable-remote-restart": {"Y", "N"},
	"enable-record-session": {"Y", "N"},
	"enable-block-input":    {"Y", "N"},
	"enable-remote-printer": {"Y", "N"},
	"access-mode":           {"full", "view", "custom"},
}

var ErrNextecPolicyInvalid = errors.New("política inválida")

func nextecPolicyKeys() []string {
	keys := make([]string, 0, len(NextecPolicyOptions))
	for k := range NextecPolicyOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func NextecPolicyList() (res []*model.NextecAppPolicy) {
	DB.Order("id desc").Find(&res)
	return res
}

// NextecPolicyUpsert cria ou altera a política de um cliente ou de uma máquina. Valor vazio = padrão do app.
func NextecPolicyUpsert(kind, ref string, enabled bool, opts map[string]string) (*model.NextecAppPolicy, error) {
	ref = strings.TrimSpace(ref)
	switch kind {
	case "group":
		g, err := strconv.Atoi(ref)
		if err != nil || g <= 0 || AllService.GroupService.DeviceGroupInfoById(uint(g)).Id == 0 {
			return nil, ErrNextecPolicyInvalid
		}
	case "peer":
		if !nextecPeerPattern.MatchString(ref) {
			return nil, ErrNextecPolicyInvalid
		}
	default:
		return nil, ErrNextecPolicyInvalid
	}
	clean := map[string]string{}
	for k, v := range opts {
		allowed, ok := NextecPolicyOptions[k]
		if !ok {
			return nil, ErrNextecPolicyInvalid
		}
		if v == "" {
			continue
		}
		valid := false
		for _, a := range allowed {
			if a == v {
				valid = true
			}
		}
		if !valid {
			return nil, ErrNextecPolicyInvalid
		}
		clean[k] = v
	}
	b, _ := json.Marshal(clean)
	p := &model.NextecAppPolicy{}
	now := time.Now().UnixMilli()
	if DB.Where("kind = ? AND ref = ?", kind, ref).First(p).Error != nil {
		p = &model.NextecAppPolicy{Kind: kind, Ref: ref}
	}
	p.Enabled, p.Options, p.ModifiedAt = enabled, string(b), now
	if err := DB.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func NextecPolicyDelete(kind, ref string) error {
	return DB.Where("kind = ? AND ref = ?", kind, ref).Delete(&model.NextecAppPolicy{}).Error
}

// nextecEffectivePolicy: regra da máquina; senão a do cliente; senão a do cliente "pai" (nomes "Cliente / Subgrupo").
func nextecEffectivePolicy(peerID string, groupID uint) *model.NextecAppPolicy {
	p := &model.NextecAppPolicy{}
	if DB.Where("kind = ? AND ref = ? AND enabled = ?", "peer", peerID, true).First(p).Error == nil {
		return p
	}
	for i := 0; groupID != 0 && i < 5; i++ {
		if DB.Where("kind = ? AND ref = ? AND enabled = ?", "group", strconv.Itoa(int(groupID)), true).First(p).Error == nil {
			return p
		}
		g := AllService.GroupService.DeviceGroupInfoById(groupID)
		idx := strings.LastIndex(g.Name, " / ")
		if g.Id == 0 || idx < 0 {
			break
		}
		parent := &model.DeviceGroup{}
		if DB.Where("name = ?", g.Name[:idx]).First(parent).Error != nil {
			break
		}
		groupID = parent.Id
	}
	return nil
}

// nextecStrategyFor monta a "strategy" do heartbeat. É declarativa: toda opção gerenciada vai, as que a política não define
// voltam ao padrão (valor vazio), assim remover uma regra devolve o app ao normal.
func nextecStrategyFor(peerID string, clientModified int64) (modifiedAt int64, opts map[string]string, send bool) {
	pr := &model.Peer{}
	DB.Select("group_id").Where("id = ?", peerID).First(pr)
	eff := nextecEffectivePolicy(peerID, pr.GroupId)
	chosen := map[string]string{}
	if eff != nil {
		if eff.ModifiedAt == clientModified {
			return 0, nil, false
		}
		modifiedAt = eff.ModifiedAt
		_ = json.Unmarshal([]byte(eff.Options), &chosen)
	} else if clientModified == 0 {
		return 0, nil, false
	}
	opts = map[string]string{}
	for _, k := range nextecPolicyKeys() {
		opts[k] = chosen[k]
	}
	return modifiedAt, opts, true
}

// ---------- conexões ativas ----------

type nextecAliveEntry struct {
	Conns []int64
	At    int64
}

var (
	nextecSessMu     sync.Mutex
	nextecAlive      = map[string]nextecAliveEntry{}
	nextecToDisconn  = map[string][]int64{}
	nextecAliveFresh = int64(90) // segundos sem heartbeat = considera encerrado
	nextecPeerOkRe   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// NextecHeartbeat registra as conexões vivas da máquina e devolve o que responder ao app (strategy, disconnect, modified_at).
func NextecHeartbeat(peerID string, conns []int64, clientModified int64) map[string]interface{} {
	resp := map[string]interface{}{}
	nextecSessMu.Lock()
	if len(conns) == 0 {
		delete(nextecAlive, peerID)
		delete(nextecToDisconn, peerID)
	} else {
		nextecAlive[peerID] = nextecAliveEntry{Conns: conns, At: time.Now().Unix()}
		if d := nextecToDisconn[peerID]; len(d) > 0 {
			resp["disconnect"] = d
			delete(nextecToDisconn, peerID)
		}
	}
	nextecSessMu.Unlock()

	if modified, opts, send := nextecStrategyFor(peerID, clientModified); send {
		resp["modified_at"] = modified
		resp["strategy"] = map[string]interface{}{"config_options": opts}
	}
	return resp
}

// NextecSession é uma conexão aberta agora.
type NextecSession struct {
	PeerId    string `json:"peer_id"`
	ConnId    int64  `json:"conn_id"`
	Alias     string `json:"alias"`
	Hostname  string `json:"hostname"`
	GroupId   uint   `json:"group_id"`
	FromName  string `json:"from_name"`
	FromPeer  string `json:"from_peer"`
	Ip        string `json:"ip"`
	Type      int    `json:"type"`
	StartedAt int64  `json:"started_at"`
	Pending   bool   `json:"pending"` // desconexão já pedida, esperando o app buscar
}

func NextecSessionsActive() []NextecSession {
	now := time.Now().Unix()
	nextecSessMu.Lock()
	type item struct {
		peer string
		conn int64
		pend bool
	}
	var items []item
	for peer, e := range nextecAlive {
		if now-e.At > nextecAliveFresh {
			delete(nextecAlive, peer)
			continue
		}
		pend := map[int64]bool{}
		for _, c := range nextecToDisconn[peer] {
			pend[c] = true
		}
		for _, c := range e.Conns {
			items = append(items, item{peer, c, pend[c]})
		}
	}
	nextecSessMu.Unlock()

	out := []NextecSession{}
	for _, it := range items {
		s := NextecSession{PeerId: it.peer, ConnId: it.conn, Pending: it.pend}
		ac := &model.AuditConn{}
		if DB.Where("peer_id = ? AND conn_id = ? AND close_time = 0", it.peer, it.conn).Order("id desc").First(ac).Error == nil {
			s.FromName, s.FromPeer, s.Ip, s.Type = ac.FromName, ac.FromPeer, ac.Ip, ac.Type
			s.StartedAt = time.Time(ac.CreatedAt).Unix()
		}
		pr := &model.Peer{}
		if DB.Where("id = ?", it.peer).First(pr).Error == nil {
			s.Alias, s.Hostname, s.GroupId = pr.Alias, pr.Hostname, pr.GroupId
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

var ErrNextecSessionGone = errors.New("conexão não está mais ativa")

// NextecSessionDisconnect pede ao app da máquina que derrube a conexão no próximo heartbeat.
func NextecSessionDisconnect(peerID string, connID int64) error {
	if !nextecPeerOkRe.MatchString(peerID) {
		return ErrNextecSessionGone
	}
	nextecSessMu.Lock()
	defer nextecSessMu.Unlock()
	e, ok := nextecAlive[peerID]
	if !ok || time.Now().Unix()-e.At > nextecAliveFresh {
		return ErrNextecSessionGone
	}
	found := false
	for _, c := range e.Conns {
		if c == connID {
			found = true
		}
	}
	if !found {
		return ErrNextecSessionGone
	}
	for _, c := range nextecToDisconn[peerID] {
		if c == connID {
			return nil
		}
	}
	nextecToDisconn[peerID] = append(nextecToDisconn[peerID], connID)
	return nil
}
