package router

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/controller/web"
	"github.com/lejianwen/rustdesk-api/v2/http/middleware"
	"net/http"
)

func WebInit(g *gin.Engine) {
	i := &web.Index{}
	g.GET("/", i.Index)

	// rotas sempre registradas; a chave app.web-client é checada a cada requisição
	wc := g.Group("/", middleware.WebClientEnabled())
	wc.GET("/webclient-config/index.js", i.ConfigJs)
	wc.StaticFS("/webclient", http.Dir(global.Config.Gin.ResourcesPath+"/web"))
	wc.StaticFS("/webclient2", http.Dir(global.Config.Gin.ResourcesPath+"/web2"))
	g.StaticFS("/_admin", http.Dir(global.Config.Gin.ResourcesPath+"/admin"))
}
