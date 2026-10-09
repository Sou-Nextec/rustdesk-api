package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecSecret: telas de administração do cofre de senhas dos servidores (somente admin).
type NextecSecret struct{}

type nextecPolicyForm struct {
	Kind     string `json:"kind" binding:"required"`
	Ref      string `json:"ref" binding:"required"`
	Enabled  bool   `json:"enabled"`
	Interval int    `json:"interval_minutes"`
	Remove   bool   `json:"remove"`
}

type nextecPeerForm struct {
	Id string `json:"id" binding:"required"`
}

func nextecIP(c *gin.Context) string {
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

// Overview devolve regras e máquinas do cofre
// @Router /admin/nextec/secrets [get]
func (n *NextecSecret) Overview(c *gin.Context) {
	response.Success(c, &gin.H{
		"agent_key_set": service.NextecAgentKeyIsSet(),
		"policies":      service.NextecSecretPolicies(),
		"devices":       service.NextecSecretDevices(),
	})
}

// Policy cria, altera ou remove a regra de um grupo ou de uma máquina
// @Router /admin/nextec/secrets/policy [post]
func (n *NextecSecret) Policy(c *gin.Context) {
	f := &nextecPolicyForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if f.Remove {
		if err := service.NextecSecretDeletePolicy(f.Kind, f.Ref); err != nil {
			response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
			return
		}
		service.NextecSecretAuditAdd(u, f.Ref, "policy_remove", nextecIP(c), f.Kind)
		response.Success(c, nil)
		return
	}
	p, err := service.NextecSecretUpsertPolicy(f.Kind, f.Ref, f.Enabled, f.Interval)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	service.NextecSecretAuditAdd(u, f.Ref, "policy_set", nextecIP(c), f.Kind)
	response.Success(c, p)
}

// RotateNow manda trocar a senha na próxima verificação do agente
// @Router /admin/nextec/secrets/rotate [post]
func (n *NextecSecret) RotateNow(c *gin.Context) {
	f := &nextecPeerForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecSecretRotateNow(f.Id); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), f.Id, "rotate_now", nextecIP(c), "")
	response.Success(c, nil)
}

// Reveal mostra a senha vigente ao admin, com registro em auditoria
// @Router /admin/nextec/secrets/reveal [post]
func (n *NextecSecret) Reveal(c *gin.Context) {
	f := &nextecPeerForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	r, err := service.NextecSecretReveal(f.Id)
	if err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), f.Id, "reveal", nextecIP(c), "")
	response.Success(c, r)
}

// Unenroll remove o agente da máquina e apaga as senhas guardadas
// @Router /admin/nextec/secrets/unenroll [post]
func (n *NextecSecret) Unenroll(c *gin.Context) {
	f := &nextecPeerForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecSecretUnenroll(f.Id); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), f.Id, "unenroll", nextecIP(c), "")
	response.Success(c, nil)
}

// AgentKey gera uma chave de cadastro nova; ela aparece só desta vez
// @Router /admin/nextec/secrets/agent-key [post]
func (n *NextecSecret) AgentKey(c *gin.Context) {
	key, err := service.NextecAgentKeyRotate()
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "agent_key_new", nextecIP(c), "")
	response.Success(c, &gin.H{"key": key})
}

// Audit lista os últimos eventos do cofre
// @Router /admin/nextec/secrets/audit [get]
func (n *NextecSecret) Audit(c *gin.Context) {
	response.Success(c, &gin.H{"list": service.NextecSecretAuditList(300)})
}
