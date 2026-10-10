package service

import (
	"strings"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// NextecCmdErrorMessage transforma o erro de rede ao falar com o hbbs ou hbbr (comandos do painel) em um texto que o
// operador entende. O erro cru ("dial tcp 127.0.0.1:21117: connect: connection refused") não diz o que fazer.
func NextecCmdErrorMessage(target string, err error) string {
	if err == nil {
		return ""
	}
	nome := "o servidor de ID (hbbs)"
	if target == model.ServerCmdTargetRelayServer {
		nome = "o relay (hbbr)"
	}
	raw := err.Error()
	low := strings.ToLower(raw)
	switch {
	case strings.Contains(low, "connection refused"):
		return "O painel não conseguiu falar com " + nome + " para enviar comandos. Ele está parado, reiniciando ou roda em outro contêiner " +
			"(o hbbs só aceita comandos vindos da mesma máquina). O acesso remoto dos dispositivos não é afetado por este aviso."
	case strings.Contains(low, "timeout") || strings.Contains(low, "deadline exceeded") || strings.Contains(low, "timed out"):
		return "O painel esperou, mas " + nome + " não respondeu ao comando. Tente de novo em instantes."
	case strings.Contains(low, "no such host") || strings.Contains(low, "network is unreachable") || strings.Contains(low, "no route to host"):
		return "O painel não encontrou " + nome + " na rede. Confira o endereço do servidor nas configurações."
	case strings.Contains(low, "eof") || strings.Contains(low, "connection reset"):
		return "A conexão com " + nome + " foi encerrada antes da resposta. Tente de novo."
	}
	return "Não foi possível enviar o comando para " + nome + ". Detalhe técnico: " + raw
}
