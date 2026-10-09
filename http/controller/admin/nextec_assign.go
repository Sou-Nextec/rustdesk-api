package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecAssign: chaves de instalação por cliente (somente admin).
type NextecAssign struct{}

// Token devolve a chave de instalação de um cliente
// @Router /admin/nextec/install-token [get]
func (n *NextecAssign) Token(c *gin.Context) {
	var q struct {
		GroupId uint `form:"group_id"`
	}
	if err := c.ShouldBindQuery(&q); err != nil || q.GroupId == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	t, err := service.NextecAssignToken(q.GroupId)
	if err != nil {
		response.Fail(c, 101, "Cliente não encontrado.")
		return
	}
	response.Success(c, &gin.H{"token": t})
}

// Revoke invalida todas as chaves de instalação distribuídas
// @Router /admin/nextec/install-token/revoke [post]
func (n *NextecAssign) Revoke(c *gin.Context) {
	if err := service.NextecAssignRevoke(); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "install_revoke", nextecIP(c), "")
	response.Success(c, nil)
}
