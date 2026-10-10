package service

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lejianwen/rustdesk-api/v2/global"
)

// Protocolo do link de conexão. O RustDesk registra no Windows o protocolo com o nome do app em minúsculas
// (um app gerado como "Nextec-Connect" abre "nextec-connect://", não "rustdesk://").
const (
	settingConnectScheme = "connect_scheme"
	defaultConnectScheme = "rustdesk"
)

var nextecSchemeRe = regexp.MustCompile(`^[a-z][a-z0-9+.-]{1,31}$`)

// nextecSchemeValid diz se o texto serve como protocolo de URL (sem "://").
func nextecSchemeValid(v string) bool {
	return nextecSchemeRe.MatchString(v)
}

// NextecConnectSchemeGet devolve o protocolo configurado (padrão rustdesk).
func NextecConnectSchemeGet() string {
	v := strings.ToLower(strings.TrimSpace(global.NextecSettingGet(settingConnectScheme)))
	if !nextecSchemeValid(v) {
		return defaultConnectScheme
	}
	return v
}

// NextecConnectSchemeSet grava o protocolo. Vazio volta ao padrão.
func NextecConnectSchemeSet(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimSuffix(v, "://")
	if v == "" {
		v = defaultConnectScheme
	}
	if !nextecSchemeValid(v) {
		return errors.New("protocolo inválido: use letras minúsculas, números e hífen, como nextec-connect")
	}
	return global.NextecSettingSet(settingConnectScheme, v)
}
