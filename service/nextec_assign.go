package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Instalação por cliente (Nextec): o comando de instalação de cada cliente leva uma chave própria (HMAC do cliente com a chave do cofre).
// Quem roda o comando a coloca na máquina no cliente certo, sem o admin mover depois. A chave só vale para esse cliente e
// pode ser invalidada (todas de uma vez) pelo admin. A máquina nunca é movida se já tiver cliente.

const (
	settingAssignEpoch = "assign_epoch"
	nextecPendingTTL   = 24 * time.Hour
)

var ErrNextecNoGroup = errors.New("cliente não encontrado")

func nextecAssignEpoch() string {
	if v := global.NextecSettingGet(settingAssignEpoch); v != "" {
		return v
	}
	return "1"
}

func nextecAssignMac(group uint) (string, error) {
	k, err := vaultKey()
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, k)
	m.Write([]byte(fmt.Sprintf("assign:%s:%d", nextecAssignEpoch(), group)))
	return hex.EncodeToString(m.Sum(nil))[:32], nil
}

// NextecAssignToken devolve a chave de instalação do cliente (grupo de dispositivos).
func NextecAssignToken(group uint) (string, error) {
	if group == 0 || AllService.GroupService.DeviceGroupInfoById(group).Id == 0 {
		return "", ErrNextecNoGroup
	}
	return nextecAssignMac(group)
}

// NextecAssignRevoke invalida todas as chaves de instalação já distribuídas.
func NextecAssignRevoke() error {
	n, _ := strconv.Atoi(nextecAssignEpoch())
	return global.NextecSettingSet(settingAssignEpoch, strconv.Itoa(n+1))
}

// NextecAssignApply é chamado pelo script de instalação. Devolve "assigned", "pending" (a máquina ainda não se registrou) ou "kept"
// (já tinha cliente, nada muda).
func NextecAssignApply(ip, peerID string, group uint, token string) (string, error) {
	if !nextecFailCheck(ip) {
		return "", ErrNextecTooManyFails
	}
	want, err := nextecAssignMac(group)
	if err != nil || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(want)) != 1 {
		nextecFailAdd(ip)
		return "", ErrNextecBadAgentAuth
	}
	if !nextecPeerPattern.MatchString(peerID) {
		return "", errors.New("id inválido")
	}
	if AllService.GroupService.DeviceGroupInfoById(group).Id == 0 {
		return "", ErrNextecNoGroup
	}
	p := AllService.PeerService.FindById(peerID)
	if p.RowId != 0 {
		if p.GroupId != 0 {
			return "kept", nil
		}
		if err := DB.Model(&model.Peer{}).Where("row_id = ?", p.RowId).Update("group_id", group).Error; err != nil {
			return "", err
		}
		return "assigned", nil
	}
	row := &model.NextecPendingAssign{PeerId: peerID, GroupId: group, ExpiresAt: time.Now().Add(nextecPendingTTL).Unix()}
	if err := DB.Save(row).Error; err != nil {
		return "", err
	}
	return "pending", nil
}

// NextecApplyPendingAssign aplica a atribuição guardada quando a máquina finalmente se registra (chamado após o sysinfo).
func NextecApplyPendingAssign(peerID string) {
	row := &model.NextecPendingAssign{}
	if DB.First(row, "peer_id = ?", peerID).Error != nil {
		return
	}
	DB.Delete(row)
	if row.ExpiresAt < time.Now().Unix() {
		return
	}
	// só atribui se ainda não tem cliente
	DB.Model(&model.Peer{}).Where("id = ? AND group_id = 0", peerID).Update("group_id", row.GroupId)
}
