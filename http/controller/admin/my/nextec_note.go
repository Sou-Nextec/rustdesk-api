package my

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecNote: chamado informado ao conectar pelo painel (qualquer usuário logado).
type NextecNote struct{}

type nextecNoteForm struct {
	Id     string `json:"id" binding:"required"`
	Ticket string `json:"ticket"`
	Note   string `json:"note"`
}

// Settings devolve se o painel pede o chamado ao conectar e o endereço base do Jira
// @Router /admin/my/connect-settings [get]
func (ct *NextecNote) Settings(c *gin.Context) {
	response.Success(c, service.NextecTicketSettingsGet())
}

// Add registra o chamado da conexão que vai começar
// @Router /admin/my/connect-note [post]
func (ct *NextecNote) Add(c *gin.Context) {
	f := &nextecNoteForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if err := service.NextecNoteAdd(u, f.Id, f.Ticket, f.Note); err != nil {
		msg := "Não foi possível registrar o chamado."
		if errors.Is(err, service.ErrNextecTicket) {
			msg = "Chamado inválido. Use letras, números e hífen, como CBQ-REQ-6054."
		}
		response.Fail(c, 101, msg)
		return
	}
	response.Success(c, nil)
}
