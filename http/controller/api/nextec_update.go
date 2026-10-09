package api

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecUpdate: endpoints públicos usados pelo script Instalar-Nextec.ps1 nas máquinas.
// O instalador não tem segredo (leva o endereço do servidor e a chave pública), mas só são servidos arquivos que o admin enviou.
type NextecUpdate struct{}

var nextecUpdateFileName = regexp.MustCompile(`^nextec-acesso-[0-9.]{5,24}\.(msi|exe)$`)

func updateIP(c *gin.Context) string {
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

// Manifest GET /api/nextec/update/versao.json?id=<ID do RustDesk>&v=<versão instalada>
// No mesmo formato do versao.json que ficava no site de atualização (windows: null = nada a instalar).
func (u *NextecUpdate) Manifest(c *gin.Context) {
	id, installed := c.Query("id"), c.Query("v")
	target := service.NextecUpdateTarget(id)
	targetVersion := ""
	out := gin.H{"windows": nil, "linux": nil}
	if target != nil {
		targetVersion = target.Version
		out["windows"] = gin.H{"versao": target.Version, "arquivo": "files/" + target.FileName, "sha256": target.Sha256}
	}
	service.NextecUpdateSeen(id, installed, targetVersion, updateIP(c))
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, out)
}

// File GET /api/nextec/update/files/:name
func (u *NextecUpdate) File(c *gin.Context) {
	name := c.Param("name")
	if !nextecUpdateFileName.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}
	r := service.NextecReleaseByFile(name)
	if r == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.FileAttachment(service.NextecReleasePath(r), r.FileName)
}
