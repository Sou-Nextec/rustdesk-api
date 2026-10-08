package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"net/http"
)

// WebClientEnabled responde 404 quando o cliente web está desligado. A checagem é feita a cada
// requisição porque a chave pode mudar em tempo de execução.
func WebClientEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		if global.Config.App.WebClient != 1 {
			// mesma resposta do NoRoute, como se a rota não existisse
			c.String(http.StatusNotFound, "404 not found")
			c.Abort()
			return
		}
		c.Next()
	}
}
