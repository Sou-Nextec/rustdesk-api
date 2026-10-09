package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Suporte avulso (Nextec): uma página pública entrega o app de suporte (gerado no rdgen) a quem ainda não tem nada instalado.
// Quando a pessoa abre o app, ele aparece na lista "Aguardando atendimento" para o técnico conectar.

const (
	nextecSupportDir        = "./data/nextec-support"
	nextecSupportFile       = "nextec-suporte.exe"
	NextecSupportDownload   = "Suporte-Nextec.exe"
	settingSupportApp       = "support_app"
	nextecWaitingOnlineSecs = int64(600)          // visto nos últimos 10 minutos
	nextecWaitingMaxAge     = 24 * time.Hour      // dispositivo novo: cadastrado nas últimas 24 horas
	nextecWaitingLimit      = 50
)

// NextecSupportApp descreve o app de suporte publicado.
type NextecSupportApp struct {
	Sha256     string `json:"sha256"`
	Size       int64  `json:"size"`
	UploadedAt int64  `json:"uploaded_at"`
	UploadedBy string `json:"uploaded_by"`
}

// NextecWaitingPeer é um dispositivo novo, sem cliente nem dono, que está online agora.
type NextecWaitingPeer struct {
	Id             string `json:"id"`
	Alias          string `json:"alias"`
	Hostname       string `json:"hostname"`
	Username       string `json:"username"`
	Os             string `json:"os"`
	CreatedAt      int64  `json:"created_at"`
	LastOnlineTime int64  `json:"last_online_time"`
}

func NextecSupportAppGet() *NextecSupportApp {
	raw := global.NextecSettingGet(settingSupportApp)
	if raw == "" {
		return nil
	}
	a := &NextecSupportApp{}
	if json.Unmarshal([]byte(raw), a) != nil || a.Sha256 == "" {
		return nil
	}
	if _, err := os.Stat(NextecSupportPath()); err != nil {
		return nil
	}
	return a
}

func NextecSupportPath() string { return filepath.Join(nextecSupportDir, nextecSupportFile) }

// NextecSupportAppSave guarda o app de suporte (.exe, até 300 MB), trocando o anterior de forma atômica.
func NextecSupportAppSave(u *model.User, originalName string, src io.Reader) (*NextecSupportApp, error) {
	if strings.ToLower(filepath.Ext(originalName)) != ".exe" {
		return nil, ErrNextecBadFile
	}
	if err := os.MkdirAll(nextecSupportDir, 0755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(nextecSupportDir, ".upload-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	head := make([]byte, 2)
	n, _ := io.ReadFull(src, head)
	if !nextecMagicOk(".exe", head[:n]) {
		tmp.Close()
		return nil, ErrNextecBadFile
	}
	h := sha256.New()
	h.Write(head[:n])
	if _, err := tmp.Write(head[:n]); err != nil {
		tmp.Close()
		return nil, err
	}
	size, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(src, NextecUpdateMaxSize+1-int64(n)))
	if err != nil {
		tmp.Close()
		return nil, err
	}
	size += int64(n)
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if size > NextecUpdateMaxSize {
		return nil, ErrNextecBadFile
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), NextecSupportPath()); err != nil {
		return nil, err
	}
	a := &NextecSupportApp{Sha256: hex.EncodeToString(h.Sum(nil)), Size: size, UploadedAt: time.Now().Unix()}
	if u != nil {
		a.UploadedBy = u.Username
	}
	b, _ := json.Marshal(a)
	if err := global.NextecSettingSet(settingSupportApp, string(b)); err != nil {
		return nil, err
	}
	return a, nil
}

func NextecSupportAppDelete() error {
	if err := global.NextecSettingSet(settingSupportApp, ""); err != nil {
		return err
	}
	if err := os.Remove(NextecSupportPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// NextecSupportWaiting lista os dispositivos novos, sem cliente nem dono, online agora (os que acabaram de abrir o app de suporte).
func NextecSupportWaiting() []NextecWaitingPeer {
	now := time.Now()
	var peers []*model.Peer
	DB.Where("group_id = 0 AND user_id = 0 AND last_online_time >= ?", now.Unix()-nextecWaitingOnlineSecs).
		Order("last_online_time desc").Limit(200).Find(&peers)
	out := []NextecWaitingPeer{}
	for _, p := range peers {
		created := time.Time(p.CreatedAt)
		if created.IsZero() || now.Sub(created) > nextecWaitingMaxAge {
			continue
		}
		out = append(out, NextecWaitingPeer{
			Id: p.Id, Alias: p.Alias, Hostname: p.Hostname, Username: p.Username, Os: p.Os,
			CreatedAt: created.Unix(), LastOnlineTime: p.LastOnlineTime,
		})
		if len(out) >= nextecWaitingLimit {
			break
		}
	}
	return out
}
