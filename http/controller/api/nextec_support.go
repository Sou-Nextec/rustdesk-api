package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// NextecSupport: página pública do suporte avulso (/suporte) e o download do app de suporte.
type NextecSupport struct{}

const supportPageTpl = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Suporte remoto Nextec</title>
<style>
:root{--bg:#f4f4f4;--card:#fff;--text:#1a1c1c;--muted:#464556;--accent:#613cb3;--on:#fff;--soft:#efecf7}
@media (prefers-color-scheme:dark){:root{--bg:#121214;--card:#1b1b1f;--text:#ececef;--muted:#bdbdc6;--accent:#b899ff;--on:#14121a;--soft:#2c2740}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px 16px;background:var(--bg);color:var(--text);font:16px/1.55 "Open Sans",system-ui,-apple-system,"Segoe UI",Roboto,Arial,sans-serif}
main{width:100%%;max-width:520px;background:var(--card);border-radius:20px;padding:32px 28px;box-shadow:0 0 32px rgba(0,0,0,.08)}
h1{margin:0 0 6px;font-size:24px;line-height:1.25}
p{margin:0 0 16px;color:var(--muted)}
ol{margin:20px 0;padding:0;list-style:none;counter-reset:s}
li{counter-increment:s;display:flex;gap:12px;margin:0 0 14px;color:var(--text)}
li::before{content:counter(s);flex:none;width:28px;height:28px;border-radius:50%%;background:var(--soft);color:var(--accent);font-weight:700;display:grid;place-items:center;font-size:14px}
a.btn{display:block;text-align:center;text-decoration:none;font-weight:700;padding:14px 18px;border-radius:12px;background:var(--accent);color:var(--on);margin:8px 0 6px}
a.btn:focus-visible{outline:3px solid var(--text);outline-offset:2px}
small{display:block;color:var(--muted);font-size:13px}
.note{margin-top:18px;padding:12px 14px;border-radius:12px;background:var(--soft);font-size:14px;color:var(--text)}
</style>
</head>
<body>
<main>
<h1>Suporte remoto Nextec</h1>
%s
</main>
</body>
</html>`

const supportPageReady = `<p>Para que a Nextec atenda você, baixe e abra o aplicativo de suporte. Leva menos de um minuto.</p>
<a class="btn" href="/suporte/baixar" download>Baixar o aplicativo de suporte</a>
<small>Windows · %s</small>
<ol>
<li><span>Abra o arquivo <strong>Suporte-Nextec.exe</strong> que foi baixado. Se o Windows mostrar um aviso, clique em <strong>Mais informações</strong> e depois em <strong>Executar assim mesmo</strong>.</span></li>
<li><span>Avise o técnico da Nextec que o aplicativo está aberto.</span></li>
<li><span>Quando ele pedir a conexão, clique em <strong>Aceitar</strong> na janela do aplicativo.</span></li>
</ol>
<div class="note">Ninguém entra no seu computador sem você aceitar. Ao final do atendimento, feche o aplicativo.</div>`

const supportPageUnavailable = `<p>Este link de suporte não está disponível no momento. Fale com a Nextec pelo canal em que você abriu o chamado.</p>`

func supportMB(n int64) string {
	return fmt.Sprintf("%.1f MB", float64(n)/1048576)
}

func supportHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
}

// Page GET /suporte
func (s *NextecSupport) Page(c *gin.Context) {
	supportHeaders(c)
	body := supportPageUnavailable
	if a := service.NextecSupportAppGet(); a != nil {
		body = fmt.Sprintf(supportPageReady, supportMB(a.Size))
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(fmt.Sprintf(supportPageTpl, body)))
}

// Download GET /suporte/baixar
func (s *NextecSupport) Download(c *gin.Context) {
	supportHeaders(c)
	if service.NextecSupportAppGet() == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.FileAttachment(service.NextecSupportPath(), service.NextecSupportDownload)
}
