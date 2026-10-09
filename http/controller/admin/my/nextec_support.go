package my

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecSupportWaiting: lista "Aguardando atendimento". Quem vê é decidido pelo admin (desligada, só administradores ou todos).
// A conexão em si sempre depende de a pessoa aceitar no app.
type NextecSupportWaiting struct{}

// List devolve os dispositivos novos online e se o app de suporte está publicado
// @Router /admin/my/support/waiting [get]
func (n *NextecSupportWaiting) List(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	if !service.NextecWaitingAllowed(u) {
		// sem permissão: a tela some com o bloco e nenhum dado de dispositivo sai
		response.Success(c, &gin.H{"list": []service.NextecWaitingPeer{}, "has_app": false, "enabled": false, "mode": service.NextecWaitingMode()})
		return
	}
	response.Success(c, &gin.H{"list": service.NextecSupportWaiting(), "has_app": service.NextecSupportAppGet() != nil, "enabled": true, "mode": service.NextecWaitingMode()})
}
