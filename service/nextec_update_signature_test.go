package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"strings"
	"testing"
	"time"
)

func TestNextecSignedRelease(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	release := &model.NextecAppRelease{Version: "2.0.3", FileName: "nextec-acesso-2.0.3.msi", Sha256: strings.Repeat("a", 64), Size: 100}
	m := nextecSignedManifest{Product: "nextec-connect", Format: 1, Sequence: 5, Version: release.Version, Issued: now, Expires: now.Add(time.Hour), Windows: &nextecSignedAsset{}}
	m.Windows.File = "files/" + release.FileName
	m.Windows.Hash = release.Sha256
	m.Windows.Size = release.Size
	sign := func(m nextecSignedManifest) NextecSignedRelease {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
		if err != nil {
			t.Fatal(err)
		}
		return NextecSignedRelease{Manifest: base64.StdEncoding.EncodeToString(raw), Signature: base64.StdEncoding.EncodeToString(signature)}
	}
	valid := sign(m)
	if err := nextecVerifySignedRelease(release, valid, &key.PublicKey, now); err != nil {
		t.Fatal(err)
	}
	linuxRelease := &model.NextecAppRelease{
		Version: release.Version, FileName: release.FileName, Sha256: release.Sha256, Size: release.Size,
		LinuxAMD64FileName: "nextec-acesso-2.0.3-amd64.deb", LinuxAMD64Sha256: strings.Repeat("b", 64), LinuxAMD64Size: 200,
	}
	m.Linux = map[string]nextecSignedAsset{"amd64": {File: "files/" + linuxRelease.LinuxAMD64FileName, Hash: linuxRelease.LinuxAMD64Sha256, Size: linuxRelease.LinuxAMD64Size}}
	if err := nextecVerifySignedRelease(linuxRelease, sign(m), &key.PublicKey, now); err != nil { t.Fatalf("signed Linux package rejected: %v", err) }
	if err := nextecVerifySignedRelease(release, valid, nextecReleaseTrustKey(), now); err == nil {
		t.Fatal("Untrusted signer accepted")
	}
	tests := []struct {
		name   string
		mutate func(*nextecSignedManifest)
	}{
		{"other product", func(m *nextecSignedManifest) { m.Product = "monitoramento" }},
		{"expired", func(m *nextecSignedManifest) { m.Expires = now.Add(-time.Second) }},
		{"future", func(m *nextecSignedManifest) { m.Issued = now.Add(time.Hour) }},
		{"long validity", func(m *nextecSignedManifest) { m.Expires = now.Add(31 * 24 * time.Hour) }},
		{"other file", func(m *nextecSignedManifest) { m.Windows.File = "files/other.exe" }},
		{"hash mismatch", func(m *nextecSignedManifest) { m.Windows.Hash = strings.Repeat("b", 64) }},
		{"size mismatch", func(m *nextecSignedManifest) { m.Windows.Size = 101 }},
		{"version mismatch", func(m *nextecSignedManifest) { m.Version = "2.0.4" }},
		{"negative sequence", func(m *nextecSignedManifest) { m.Sequence = -1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := m
			tc.mutate(&bad)
			if nextecVerifySignedRelease(release, sign(bad), &key.PublicKey, now) == nil {
				t.Fatal("Invalid manifest accepted")
			}
		})
	}
	raw, _ := base64.StdEncoding.DecodeString(valid.Manifest)
	valid.Manifest = base64.StdEncoding.EncodeToString(append(raw, ' '))
	if nextecVerifySignedRelease(release, valid, &key.PublicKey, now) == nil {
		t.Fatal("Tampered bytes accepted")
	}
}

