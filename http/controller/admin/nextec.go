package admin

import (
	"encoding/json"

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

type nextecTemplatesForm struct {
	Templates json.RawMessage `json:"templates" binding:"required"`
}

const nextecTemplatesMaxBytes = 256 * 1024

// ClientTemplates devolve os modelos de cliente
// @Tags ADMIN
// @Summary Modelos de cliente (subgrupos e permissões)
// @Produce  json
// @Success 200 {object} response.Response
// @Router /admin/nextec/client-templates [get]
// @Security token
func (n *Nextec) ClientTemplates(c *gin.Context) {
	v, err := global.NextecClientTemplates()
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, &gin.H{"templates": v})
}

// SaveClientTemplates grava os modelos de cliente (lista JSON, até 256 KB)
// @Tags ADMIN
// @Summary Salva os modelos de cliente
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Router /admin/nextec/client-templates [post]
// @Security token
func (n *Nextec) SaveClientTemplates(c *gin.Context) {
	f := &nextecTemplatesForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	var list []json.RawMessage
	if len(f.Templates) > nextecTemplatesMaxBytes || json.Unmarshal(f.Templates, &list) != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	if err := global.SetNextecClientTemplates(f.Templates); err != nil {
		global.Logger.Error("save nextec templates: ", err)
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}
