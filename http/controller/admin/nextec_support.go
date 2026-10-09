package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecSupport: administração do app de suporte avulso (somente admin).
type NextecSupport struct{}

// Info devolve o app publicado e o caminho público da página
// @Router /admin/nextec/support [get]
func (n *NextecSupport) Info(c *gin.Context) {
	response.Success(c, &gin.H{"app": service.NextecSupportAppGet(), "path": "/suporte", "max_size": service.NextecUpdateMaxSize, "waiting_mode": service.NextecWaitingMode()})
}

type nextecSupportSettingsForm struct {
	WaitingMode string `json:"waiting_mode" binding:"required"`
}

// Settings define quem vê a fila Aguardando atendimento (off, admins ou all)
// @Router /admin/nextec/support/settings [post]
func (n *NextecSupport) Settings(c *gin.Context) {
	f := &nextecSupportSettingsForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if err := service.NextecWaitingModeSet(f.WaitingMode); err != nil {
		response.Fail(c, 101, "Opção inválida. Use off, admins ou all.")
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "support_settings", nextecIP(c), f.WaitingMode)
	response.Success(c, &gin.H{"waiting_mode": service.NextecWaitingMode()})
}

// Upload recebe o app de suporte (multipart: file)
// @Router /admin/nextec/support/upload [post]
func (n *NextecSupport) Upload(c *gin.Context) {
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
	a, err := service.NextecSupportAppSave(u, fh.Filename, f)
	if err != nil {
		response.Fail(c, 101, "Arquivo inválido. Envie o aplicativo de suporte (.exe), com até 300 MB.")
		return
	}
	service.NextecSecretAuditAdd(u, "", "support_upload", nextecIP(c), "")
	response.Success(c, a)
}

// Delete tira o app de suporte do ar
// @Router /admin/nextec/support/delete [post]
func (n *NextecSupport) Delete(c *gin.Context) {
	if err := service.NextecSupportAppDelete(); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	service.NextecSecretAuditAdd(service.AllService.UserService.CurUser(c), "", "support_delete", nextecIP(c), "")
	response.Success(c, nil)
}
