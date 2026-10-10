package my

import (
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecConnect monta o link de conexão do RustDesk. Em máquinas com senha automática, o link já leva a senha vigente,
// e só para quem pode acessar a máquina; cada uso fica registrado em auditoria.
type NextecConnect struct{}

// Link devolve <protocolo>://<id>[?password=...] (protocolo: rustdesk ou o nome do app gerado, ver NextecConnectSchemeGet)
// @Router /admin/my/connect-link [get]
func (ct *NextecConnect) Link(c *gin.Context) {
	id := c.Query("id")
	if id == "" || len(id) > 64 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	u := service.AllService.UserService.CurUser(c)
	pw, allowed := service.NextecConnectPassword(u, id)
	if !allowed {
		response.Fail(c, 403, response.TranslateMsg(c, "NoAccess"))
		return
	}
	link := service.NextecConnectSchemeGet() + "://" + url.PathEscape(id)
	if pw != "" {
		link += "?password=" + url.QueryEscape(pw)
		ip := c.GetHeader("CF-Connecting-IP")
		if ip == "" {
			ip = c.ClientIP()
		}
		service.NextecSecretAuditAdd(u, id, "connect", ip, "")
	}
	response.Success(c, &gin.H{"url": link, "auto_password": pw != ""})
}
