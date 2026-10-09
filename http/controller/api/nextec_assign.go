package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecAssign: chamado pelo script de instalação para colocar a máquina no cliente do comando.
type NextecAssign struct{}

type nextecAssignForm struct {
	Id    string `json:"id"`
	Group uint   `json:"group"`
	Token string `json:"token"`
}

// Assign POST /api/nextec/install/assign {id, group, token}
func (a *NextecAssign) Assign(c *gin.Context) {
	f := &nextecAssignForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	state, err := service.NextecAssignApply(agentIP(c), f.Id, f.Group, f.Token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNextecTooManyFails):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failures"})
		case errors.Is(err, service.ErrNextecBadAgentAuth):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		case errors.Is(err, service.ErrNextecNoGroup):
			c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"state": state})
}
