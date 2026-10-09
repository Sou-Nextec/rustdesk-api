package service

import (
	"regexp"
	"strings"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// ID do RustDesk sem espaços (Nextec): o painel mostra "268 304 385", mas o ID de verdade é "268304385".
// IDs digitados à mão com espaço nunca casavam com o que o app envia; aqui eles são normalizados ao gravar
// e uma vez na inicialização para o que já estava gravado.

var nextecIdSpaces = regexp.MustCompile(`[\s\x{00a0}]+`)

// NextecCleanId tira espaços (inclusive o sem quebra) do ID.
func NextecCleanId(id string) string {
	return nextecIdSpaces.ReplaceAllString(strings.TrimSpace(id), "")
}

// NextecNormalizeIds corrige IDs já gravados com espaço (dispositivos e acessos salvos) quando não há conflito.
func NextecNormalizeIds() {
	var peers []*model.Peer
	DB.Where("id LIKE '% %' OR id LIKE ?", "%\u00a0%").Find(&peers)
	fixed, skipped := 0, 0
	for _, p := range peers {
		nid := NextecCleanId(p.Id)
		if nid == "" || nid == p.Id {
			continue
		}
		var n int64
		DB.Model(&model.Peer{}).Where("id = ?", nid).Count(&n)
		if n > 0 {
			skipped++
			Logger.Warn("nextec: ID com espaço não corrigido por conflito: ", p.Id)
			continue
		}
		if err := DB.Model(&model.Peer{}).Where("row_id = ?", p.RowId).Update("id", nid).Error; err != nil {
			Logger.Warn("nextec: falha ao corrigir ID: ", err)
			continue
		}
		fixed++
	}
	var abs []*model.AddressBook
	DB.Where("id LIKE '% %' OR id LIKE ?", "%\u00a0%").Find(&abs)
	for _, a := range abs {
		nid := NextecCleanId(a.Id)
		if nid == "" || nid == a.Id {
			continue
		}
		var n int64
		DB.Model(&model.AddressBook{}).Where("id = ? AND user_id = ? AND collection_id = ?", nid, a.UserId, a.CollectionId).Count(&n)
		if n > 0 {
			skipped++
			continue
		}
		if err := DB.Model(&model.AddressBook{}).Where("row_id = ?", a.RowId).Update("id", nid).Error; err == nil {
			fixed++
		}
	}
	if fixed > 0 || skipped > 0 {
		Logger.Info("nextec: IDs com espaço corrigidos: ", fixed, ", ignorados por conflito: ", skipped)
	}
}
