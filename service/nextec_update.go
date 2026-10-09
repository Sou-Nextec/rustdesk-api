package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Atualização do app pelo painel (Nextec): o administrador envia o instalador, escolhe quem recebe
// (todos ou um grupo piloto) e as máquinas, pelo script Instalar-Nextec.ps1, buscam e instalam sozinhas.

const (
	nextecUpdateDir     = "./data/nextec-updates"
	NextecUpdateMaxSize = int64(300 << 20) // 300 MB
	settingAppRollout   = "app_rollout"

	NextecRolloutOff   = "off"
	NextecRolloutPilot = "pilot"
	NextecRolloutAll   = "all"
)

var (
	ErrNextecBadVersion   = errors.New("invalid version")
	ErrNextecBadFile      = errors.New("invalid file")
	ErrNextecVersionTaken = errors.New("version already exists")
	ErrNextecInUse        = errors.New("release is published")
	ErrNextecNoRelease    = errors.New("release not found")
	nextecVersionPattern  = regexp.MustCompile(`^\d{1,4}\.\d{1,4}\.\d{1,4}(\.\d{1,4})?$`)
	nextecPeerPattern     = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// NextecRollout diz qual versão está publicada e para quem.
type NextecRollout struct {
	Version string   `json:"version"`
	Mode    string   `json:"mode"` // off | pilot | all
	Groups  []uint   `json:"groups"`
	Peers   []string `json:"peers"`
}

func NextecRolloutGet() NextecRollout {
	r := NextecRollout{Mode: NextecRolloutOff, Groups: []uint{}, Peers: []string{}}
	if raw := global.NextecSettingGet(settingAppRollout); raw != "" {
		_ = json.Unmarshal([]byte(raw), &r)
	}
	if r.Groups == nil {
		r.Groups = []uint{}
	}
	if r.Peers == nil {
		r.Peers = []string{}
	}
	if r.Mode != NextecRolloutPilot && r.Mode != NextecRolloutAll {
		r.Mode = NextecRolloutOff
	}
	return r
}

// NextecRolloutSet publica uma versão para todos, para um piloto ou suspende a publicação.
func NextecRolloutSet(version, mode string, groups []uint, peers []string) error {
	r := NextecRollout{Mode: mode, Groups: []uint{}, Peers: []string{}}
	switch mode {
	case NextecRolloutOff:
		// mantém a versão anterior só como referência
		r.Version = NextecRolloutGet().Version
	case NextecRolloutPilot, NextecRolloutAll:
		if NextecReleaseByVersion(version) == nil {
			return ErrNextecNoRelease
		}
		r.Version = version
		if mode == NextecRolloutPilot {
			seen := map[uint]bool{}
			for _, g := range groups {
				if g != 0 && !seen[g] {
					seen[g] = true
					r.Groups = append(r.Groups, g)
				}
			}
			for _, p := range peers {
				p = strings.ReplaceAll(strings.TrimSpace(p), " ", "")
				if nextecPeerPattern.MatchString(p) {
					r.Peers = append(r.Peers, p)
				}
			}
			if len(r.Groups) == 0 && len(r.Peers) == 0 {
				return errors.New("pilot needs at least one group or machine")
			}
		}
	default:
		return errors.New("invalid mode")
	}
	b, _ := json.Marshal(r)
	return global.NextecSettingSet(settingAppRollout, string(b))
}

func NextecReleaseList() (res []*model.NextecAppRelease) {
	DB.Order("id desc").Find(&res)
	return res
}

func NextecReleaseByVersion(v string) *model.NextecAppRelease {
	r := &model.NextecAppRelease{}
	if DB.Where("version = ?", v).First(r).Error != nil {
		return nil
	}
	return r
}

func NextecReleaseByFile(name string) *model.NextecAppRelease {
	r := &model.NextecAppRelease{}
	if DB.Where("file_name = ?", name).First(r).Error != nil {
		return nil
	}
	return r
}

func NextecReleasePath(r *model.NextecAppRelease) string {
	return filepath.Join(nextecUpdateDir, r.FileName)
}

// NextecReleaseSave guarda o instalador enviado: confere versão, extensão e cabeçalho do arquivo, calcula o SHA-256
// e nunca sobrescreve uma versão existente.
func NextecReleaseSave(u *model.User, version, notes, originalName string, src io.Reader) (*model.NextecAppRelease, error) {
	version = strings.TrimSpace(version)
	if !nextecVersionPattern.MatchString(version) {
		return nil, ErrNextecBadVersion
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext != ".msi" && ext != ".exe" {
		return nil, ErrNextecBadFile
	}
	if NextecReleaseByVersion(version) != nil {
		return nil, ErrNextecVersionTaken
	}
	if err := os.MkdirAll(nextecUpdateDir, 0755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(nextecUpdateDir, ".upload-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	head := make([]byte, 8)
	n, _ := io.ReadFull(src, head)
	head = head[:n]
	if !nextecMagicOk(ext, head) {
		tmp.Close()
		return nil, ErrNextecBadFile
	}
	h.Write(head)
	if _, err := tmp.Write(head); err != nil {
		tmp.Close()
		return nil, err
	}
	// um byte a mais que o limite para detectar arquivo grande demais
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
	name := fmt.Sprintf("nextec-acesso-%s%s", version, ext)
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(nextecUpdateDir, name)); err != nil {
		return nil, err
	}
	r := &model.NextecAppRelease{
		Version: version, FileName: name, Sha256: hex.EncodeToString(h.Sum(nil)), Size: size,
		Notes: truncate(strings.TrimSpace(notes), 1000), CreatedAt: time.Now().Unix(),
	}
	if u != nil {
		r.UploadedBy = u.Username
	}
	if err := DB.Create(r).Error; err != nil {
		_ = os.Remove(filepath.Join(nextecUpdateDir, name))
		return nil, err
	}
	return r, nil
}

func nextecMagicOk(ext string, head []byte) bool {
	switch ext {
	case ".msi":
		return len(head) >= 8 && string(head[:8]) == "\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"
	case ".exe":
		return len(head) >= 2 && head[0] == 'M' && head[1] == 'Z'
	}
	return false
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// NextecReleaseDelete apaga uma versão que não esteja publicada.
func NextecReleaseDelete(id uint) error {
	r := &model.NextecAppRelease{}
	if DB.First(r, id).Error != nil {
		return ErrNextecNoRelease
	}
	if ro := NextecRolloutGet(); ro.Mode != NextecRolloutOff && ro.Version == r.Version {
		return ErrNextecInUse
	}
	if err := DB.Delete(r).Error; err != nil {
		return err
	}
	_ = os.Remove(NextecReleasePath(r))
	return nil
}

// NextecUpdateTarget devolve a versão que esta máquina deve instalar (nil se nenhuma).
func NextecUpdateTarget(peerID string) *model.NextecAppRelease {
	ro := NextecRolloutGet()
	if ro.Mode == NextecRolloutOff || ro.Version == "" {
		return nil
	}
	if ro.Mode == NextecRolloutPilot {
		inPilot := false
		for _, p := range ro.Peers {
			if p == peerID {
				inPilot = true
			}
		}
		if !inPilot && peerID != "" {
			if p := AllService.PeerService.FindById(peerID); p != nil && p.RowId != 0 && p.GroupId != 0 {
				for _, g := range ro.Groups {
					if g == p.GroupId {
						inPilot = true
					}
				}
			}
		}
		if !inPilot {
			return nil
		}
	}
	return NextecReleaseByVersion(ro.Version)
}

// NextecUpdateSeen registra a checagem de uma máquina conhecida (ignora IDs que não existem, para não encher o banco).
func NextecUpdateSeen(peerID, installed, target, ip string) {
	if !nextecPeerPattern.MatchString(peerID) {
		return
	}
	if p := AllService.PeerService.FindById(peerID); p == nil || p.RowId == 0 {
		return
	}
	if installed != "" && !nextecVersionPattern.MatchString(installed) {
		installed = ""
	}
	row := &model.NextecAppInstall{PeerId: peerID}
	if DB.First(row, "peer_id = ?", peerID).Error != nil {
		row.Version, row.Target, row.SeenAt, row.Ip = installed, target, time.Now().Unix(), ip
		DB.Create(row)
		return
	}
	updates := map[string]interface{}{"target": target, "seen_at": time.Now().Unix(), "ip": ip}
	if installed != "" {
		updates["version"] = installed
	}
	DB.Model(&model.NextecAppInstall{}).Where("peer_id = ?", peerID).Updates(updates)
}

func NextecUpdateInstalls() (res []*model.NextecAppInstall) {
	DB.Order("seen_at desc").Limit(5000).Find(&res)
	return res
}
