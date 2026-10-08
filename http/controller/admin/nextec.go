package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
)

type Nextec struct {
}

type NextecWebClientForm struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// WebClient liga ou desliga o cliente web
// @Tags ADMIN
// @Summary Liga ou desliga o cliente web
// @Description Altera app.web-client em tempo de execução e grava em data/nextec-settings.json
// @Accept  json
// @Produce  json
// @Param body body NextecWebClientForm true "enabled"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/nextec/web-client [post]
// @Security token
func (n *Nextec) WebClient(c *gin.Context) {
	f := &NextecWebClientForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := global.SetWebClient(*f.Enabled); err != nil {
		global.Logger.Error("save nextec settings: ", err)
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, &gin.H{
		"web_client": global.Config.App.WebClient,
	})
}
