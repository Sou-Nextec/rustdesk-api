package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecReport: ajustes do chamado e relatório mensal por cliente (somente admin).
type NextecReport struct{}

type nextecTicketSettingsForm struct {
	Mode     string `json:"mode" binding:"required"`
	JiraBase string `json:"jira_base"`
}

// TicketSettings @Router /admin/nextec/ticket-settings [get]
func (n *NextecReport) TicketSettings(c *gin.Context) {
	response.Success(c, service.NextecTicketSettingsGet())
}

// TicketSettingsSave @Router /admin/nextec/ticket-settings [post]
func (n *NextecReport) TicketSettingsSave(c *gin.Context) {
	f := &nextecTicketSettingsForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecTicketSettingsSet(f.Mode, f.JiraBase); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "ticket_settings", nextecIP(c), f.Mode)
	response.Success(c, service.NextecTicketSettingsGet())
}

// Report @Router /admin/nextec/report [get]
func (n *NextecReport) Report(c *gin.Context) {
	var q struct {
		Month   string `form:"month"`
		GroupId uint   `form:"group_id"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	rows, truncated, err := service.NextecReportMonth(q.Month, q.GroupId)
	if err != nil {
		msg := "Não foi possível montar o relatório."
		if errors.Is(err, service.ErrNextecMonth) {
			msg = "Mês inválido. Use o formato AAAA-MM."
		}
		response.Fail(c, 101, msg)
		return
	}
	response.Success(c, &gin.H{"list": rows, "truncated": truncated})
}
