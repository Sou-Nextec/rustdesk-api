package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecControl: políticas do app e conexões ativas (somente admin).
type NextecControl struct{}

type nextecAppPolicyForm struct {
	Kind    string            `json:"kind" binding:"required"`
	Ref     string            `json:"ref" binding:"required"`
	Enabled bool              `json:"enabled"`
	Options map[string]string `json:"options"`
	Remove  bool              `json:"remove"`
}

type nextecDisconnectForm struct {
	PeerId string `json:"peer_id" binding:"required"`
	ConnId int64  `json:"conn_id"`
}

// Policies lista as políticas e as opções que o painel gerencia
// @Router /admin/nextec/policies [get]
func (n *NextecControl) Policies(c *gin.Context) {
	opts := map[string][]string{}
	for k, v := range service.NextecPolicyOptions {
		opts[k] = v
	}
	response.Success(c, &gin.H{"list": service.NextecPolicyList(), "options": opts})
}

// PolicySave cria, altera ou remove uma política
// @Router /admin/nextec/policies [post]
func (n *NextecControl) PolicySave(c *gin.Context) {
	f := &nextecAppPolicyForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if f.Remove {
		if err := service.NextecPolicyDelete(f.Kind, f.Ref); err != nil {
			response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
			return
		}
		service.NextecSecretAuditAdd(u, f.Kind+":"+f.Ref, "app_policy_remove", nextecIP(c), "")
		response.Success(c, nil)
		return
	}
	p, err := service.NextecPolicyUpsert(f.Kind, f.Ref, f.Enabled, f.Options)
	if err != nil {
		response.Fail(c, 101, "Política inválida. Confira o alvo e os valores.")
		return
	}
	service.NextecSecretAuditAdd(u, f.Kind+":"+f.Ref, "app_policy_set", nextecIP(c), p.Options)
	response.Success(c, p)
}

// Sessions lista as conexões abertas agora
// @Router /admin/nextec/sessions [get]
func (n *NextecControl) Sessions(c *gin.Context) {
	response.Success(c, &gin.H{"list": service.NextecSessionsActive()})
}

// Disconnect pede ao app que derrube uma conexão
// @Router /admin/nextec/sessions/disconnect [post]
func (n *NextecControl) Disconnect(c *gin.Context) {
	f := &nextecDisconnectForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecSessionDisconnect(f.PeerId, f.ConnId); err != nil {
		msg := "Não foi possível desconectar."
		if errors.Is(err, service.ErrNextecSessionGone) {
			msg = "Essa conexão não está mais ativa."
		}
		response.Fail(c, 101, msg)
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), f.PeerId, "session_disconnect", nextecIP(c), "")
	response.Success(c, nil)
}
