package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecUpdate: tela de atualização do app pelo painel (somente admin).
type NextecUpdate struct{}

// Signature authorizes an already uploaded MSI; the private key stays on the publisher's computer.
func (n *NextecUpdate) Signature(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100000)
	var form struct {
		Version   string `json:"version"`
		Manifest  string `json:"manifesto"`
		Signature string `json:"assinatura"`
	}
	if c.ShouldBindJSON(&form) != nil {
		response.Fail(c, 101, service.ErrNextecReleaseSignature.Error())
		return
	}
	if err := service.NextecReleaseSignatureSave(form.Version, service.NextecSignedRelease{Manifest: form.Manifest, Signature: form.Signature}); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "update_authorize", nextecIP(c), form.Version)
	response.Success(c, nil)
}

type nextecRolloutForm struct {
	Version string   `json:"version"`
	Mode    string   `json:"mode" binding:"required"`
	Groups  []uint   `json:"groups"`
	Peers   []string `json:"peers"`
}

type nextecReleaseIdForm struct {
	Id uint `json:"id" binding:"required"`
}

func nextecUpdateMsg(err error) string {
	switch {
	case errors.Is(err, service.ErrNextecBadVersion):
		return "Versão inválida. Use números separados por ponto, como 2.0.1."
	case errors.Is(err, service.ErrNextecBadFile):
		return "Arquivo inválido. Envie o MSI ou um pacote Linux .deb de arquitetura suportada, com até 300 MB."
	case errors.Is(err, service.ErrNextecVersionTaken):
		return "Essa versão já foi enviada. Use um número novo."
	case errors.Is(err, service.ErrNextecInUse):
		return "Essa versão está publicada. Suspenda ou troque a publicação antes de excluir."
	case errors.Is(err, service.ErrNextecNoRelease):
		return "Versão não encontrada."
	}
	return err.Error()
}

// Overview devolve versões enviadas, publicação atual e o que cada máquina informou
// @Router /admin/nextec/updates [get]
func (n *NextecUpdate) Overview(c *gin.Context) {
	response.Success(c, &gin.H{
		"releases": service.NextecReleaseList(),
		"rollout":  service.NextecRolloutGet(),
		"installs": service.NextecUpdateInstalls(),
		"max_size": service.NextecUpdateMaxSize,
	})
}

// Upload recebe o instalador (multipart: file, version, notes)
// @Router /admin/nextec/updates/upload [post]
func (n *NextecUpdate) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.NextecUpdateMaxSize+(2<<20))
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, 101, nextecUpdateMsg(service.ErrNextecBadFile))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, 101, nextecUpdateMsg(service.ErrNextecBadFile))
		return
	}
	defer f.Close()
	u := service.AllService.UserService.CurUser(c)
	r, err := service.NextecReleaseSave(u, c.PostForm("version"), c.PostForm("notes"), fh.Filename, c.PostForm("platform"), f)
	if err != nil {
		response.Fail(c, 101, nextecUpdateMsg(err))
		return
	}
	service.NextecSecretAuditAdd(u, "", "update_upload", nextecIP(c), r.Version)
	response.Success(c, r)
}

// Rollout publica uma versão para todos ou para um piloto, ou suspende
// @Router /admin/nextec/updates/rollout [post]
func (n *NextecUpdate) Rollout(c *gin.Context) {
	f := &nextecRolloutForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecRolloutSet(f.Version, f.Mode, f.Groups, f.Peers); err != nil {
		response.Fail(c, 101, nextecUpdateMsg(err))
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "update_publish", nextecIP(c), f.Mode+" "+f.Version)
	response.Success(c, service.NextecRolloutGet())
}

// Delete apaga uma versão que não está publicada
// @Router /admin/nextec/updates/delete [post]
func (n *NextecUpdate) Delete(c *gin.Context) {
	f := &nextecReleaseIdForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecReleaseDelete(f.Id); err != nil {
		response.Fail(c, 101, nextecUpdateMsg(err))
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "update_delete", nextecIP(c), "")
	response.Success(c, nil)
}

