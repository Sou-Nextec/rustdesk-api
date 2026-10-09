package my

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecSupportWaiting: lista "Aguardando atendimento", visível a qualquer usuário logado do painel
// (quem atende precisa ver quem acabou de abrir o app de suporte; a conexão em si depende de a pessoa aceitar no app).
type NextecSupportWaiting struct{}

// List devolve os dispositivos novos online e se o app de suporte está publicado
// @Router /admin/my/support/waiting [get]
func (n *NextecSupportWaiting) List(c *gin.Context) {
	response.Success(c, &gin.H{"list": service.NextecSupportWaiting(), "has_app": service.NextecSupportAppGet() != nil})
}
