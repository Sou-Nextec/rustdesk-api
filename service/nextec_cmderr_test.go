package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

func TestNextecCmdErrorMessage(t *testing.T) {
	cases := []struct {
		target string
		err    string
		want   string
	}{
		{model.ServerCmdTargetRelayServer, "dial tcp 127.0.0.1:21117: connect: connection refused", "relay (hbbr)"},
		{model.ServerCmdTargetIdServer, "dial tcp 127.0.0.1:21115: connect: connection refused", "servidor de ID (hbbs)"},
		{model.ServerCmdTargetIdServer, "dial tcp 10.0.0.9:21115: i/o timeout", "não respondeu"},
		{model.ServerCmdTargetIdServer, "dial tcp: lookup servidor: no such host", "não encontrou"},
		{model.ServerCmdTargetRelayServer, "read tcp 1.2.3.4:5: connection reset by peer", "encerrada"},
		{model.ServerCmdTargetRelayServer, "algo inesperado", "Detalhe técnico: algo inesperado"},
	}
	for _, c := range cases {
		got := NextecCmdErrorMessage(c.target, errors.New(c.err))
		if !strings.Contains(got, c.want) {
			t.Fatalf("%q: mensagem %q não contém %q", c.err, got, c.want)
		}
		if strings.Contains(got, "dial tcp") && !strings.Contains(c.err, "algo inesperado") {
			t.Fatalf("%q: a mensagem não deveria expor o erro cru: %q", c.err, got)
		}
	}
	if NextecCmdErrorMessage(model.ServerCmdTargetIdServer, nil) != "" {
		t.Fatal("erro nulo deve dar texto vazio")
	}
}
