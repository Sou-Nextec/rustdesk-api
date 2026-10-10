package service

import "testing"

func TestNextecSchemeValid(t *testing.T) {
	ok := []string{"rustdesk", "nextec-connect", "nextec.connect", "app2"}
	bad := []string{"", "r", "Nextec-Connect", "nextec connect", "1app", "javascript:alert(1)", "nextec://", "a/b", "-app"}
	for _, v := range ok {
		if !nextecSchemeValid(v) {
			t.Fatalf("%q deveria ser válido", v)
		}
	}
	for _, v := range bad {
		if nextecSchemeValid(v) {
			t.Fatalf("%q deveria ser inválido", v)
		}
	}
}
