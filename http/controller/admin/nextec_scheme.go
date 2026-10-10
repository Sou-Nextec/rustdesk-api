package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecScheme: protocolo do link do botão Conectar (somente admin).
type NextecScheme struct{}

type nextecSchemeForm struct {
	Scheme string `json:"scheme"`
}

// Get @Router /admin/nextec/connect-scheme [get]
func (n *NextecScheme) Get(c *gin.Context) {
	response.Success(c, &gin.H{"scheme": service.NextecConnectSchemeGet()})
}

// Save @Router /admin/nextec/connect-scheme [post]
func (n *NextecScheme) Save(c *gin.Context) {
	f := &nextecSchemeForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecConnectSchemeSet(f.Scheme); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "connect_scheme", nextecIP(c), service.NextecConnectSchemeGet())
	response.Success(c, &gin.H{"scheme": service.NextecConnectSchemeGet()})
}
