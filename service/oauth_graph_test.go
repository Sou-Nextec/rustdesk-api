package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestFetchGraphPhoto(t *testing.T) {
	logger := log.New()
	logger.SetOutput(io.Discard)
	Logger = logger
	foto := []byte{0xff, 0xd8, 0xff, 0xe0, 1, 2, 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(foto)
		case "/semfoto":
			w.WriteHeader(http.StatusNotFound)
		case "/grande":
			w.Write(make([]byte, 200*1024))
		}
	}))
	defer srv.Close()

	if got := fetchGraphPhoto(srv.Client(), srv.URL+"/ok"); !strings.HasPrefix(got, "data:image/jpeg;base64,") {
		t.Fatalf("esperava data URI, veio %q", got)
	}
	if got := fetchGraphPhoto(srv.Client(), srv.URL+"/semfoto"); got != "" {
		t.Fatalf("404 deveria devolver vazio, veio %q", got)
	}
	if got := fetchGraphPhoto(srv.Client(), srv.URL+"/grande"); got != "" {
		t.Fatalf("foto acima do limite deveria devolver vazio")
	}
	if got := fetchGraphPhoto(nil, srv.URL+"/ok"); got != "" {
		t.Fatalf("cliente nulo deveria devolver vazio")
	}
}

func TestAvatarCanBeReplaced(t *testing.T) {
	casos := map[string]bool{
		"": true,
		"https://graph.microsoft.com/v1.0/me/photo/$value": true,
		"data:image/jpeg;base64,AAAA":                      false,
		"https://exemplo.com/foto.png":                     false,
	}
	for avatar, esperado := range casos {
		if got := avatarCanBeReplaced(avatar); got != esperado {
			t.Errorf("avatar %q: esperado %v, veio %v", avatar, esperado, got)
		}
	}
}
