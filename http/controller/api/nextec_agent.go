package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecAgent: endpoints chamados pelo agente instalado em cada máquina (autenticam por chave de cadastro e por token da máquina).
type NextecAgent struct{}

type nextecAgentForm struct {
	Key      string `json:"key"`
	Id       string `json:"id"`
	Token    string `json:"token"`
	Password string `json:"password"`
}

func agentIP(c *gin.Context) string {
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

func agentFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNextecTooManyFails):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failures"})
	case errors.Is(err, service.ErrNextecAlreadyEnroll):
		c.JSON(http.StatusConflict, gin.H{"error": "already enrolled"})
	case errors.Is(err, service.ErrNextecBadPassword), errors.Is(err, service.ErrNextecNoPending):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNextecBadAgentAuth):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// Enroll POST /api/nextec/agent/enroll {key, id}
func (a *NextecAgent) Enroll(c *gin.Context) {
	f := &nextecAgentForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	token, err := service.NextecAgentEnroll(agentIP(c), f.Key, f.Id)
	if err != nil {
		agentFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Sync POST /api/nextec/agent/sync {id, token}
func (a *NextecAgent) Sync(c *gin.Context) {
	f := &nextecAgentForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	s, err := service.NextecAgentSyncState(agentIP(c), f.Id, f.Token)
	if err != nil {
		agentFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"managed": s.Managed, "rotate": s.Rotate, "interval_minutes": s.Interval,
		// ajustes que o agente aplica no RustDesk para entrar só com a senha, sem alguém aprovar
		"approve_mode": "password", "verification_method": "use-permanent-password",
	})
}

// Password POST /api/nextec/agent/password {id, token, password}: guarda a senha nova como pendente
func (a *NextecAgent) Password(c *gin.Context) {
	f := &nextecAgentForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if err := service.NextecAgentPropose(agentIP(c), f.Id, f.Token, f.Password); err != nil {
		agentFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Confirm POST /api/nextec/agent/confirm {id, token}: a senha pendente passa a valer
func (a *NextecAgent) Confirm(c *gin.Context) {
	f := &nextecAgentForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if err := service.NextecAgentConfirm(agentIP(c), f.Id, f.Token); err != nil {
		agentFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
