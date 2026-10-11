package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecUpdate: endpoints públicos usados pelo script Instalar-Nextec.ps1 nas máquinas.
// O instalador não tem segredo (leva o endereço do servidor e a chave pública), mas só são servidos arquivos que o admin enviou.
type NextecUpdate struct{}

var nextecUpdateFileName = regexp.MustCompile(`^nextec-acesso-[0-9.]{5,24}(?:-(amd64|arm64))?\.(msi|exe|deb)$`)

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
	if signed := service.NextecReleaseSignature(target); signed != nil {
		var manifest struct {
			Windows *struct { File string `json:"arquivo"`; Hash string `json:"sha256"`; Size int64 `json:"tamanho"` } `json:"windows"`
			Linux map[string]struct { File string `json:"arquivo"`; Hash string `json:"sha256"`; Size int64 `json:"tamanho"` } `json:"linux"`
		}
		raw, err := base64.StdEncoding.DecodeString(signed.Manifest)
		if err == nil && json.Unmarshal(raw, &manifest) == nil {
			if c.Query("platform") == "linux" {
				arch := c.Query("arch")
				if (arch == "amd64" || arch == "arm64") {
					expectedFile := target.LinuxAMD64FileName
					if arch == "arm64" { expectedFile = target.LinuxARM64FileName }
					if asset, ok := manifest.Linux[arch]; ok && expectedFile != "" && asset.File == "files/"+expectedFile {
						targetVersion = target.Version
						out["linux"] = gin.H{"versao": target.Version, "arquivo": asset.File, "sha256": asset.Hash, "tamanho": asset.Size, "manifesto": signed.Manifest, "assinatura": signed.Signature}
					}
				}
			} else if manifest.Windows != nil && target.FileName != "" {
				targetVersion = target.Version
				out["windows"] = gin.H{"versao": target.Version, "arquivo": manifest.Windows.File, "sha256": manifest.Windows.Hash, "tamanho": manifest.Windows.Size, "manifesto": signed.Manifest, "assinatura": signed.Signature}
			}
		}
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
	path := service.NextecReleasePathForFile(r, name)
	if path == "" { c.Status(http.StatusNotFound); return }
	c.FileAttachment(path, name)
}

