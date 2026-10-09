package model

// NextecAppPolicy é uma política das opções do app RustDesk, por cliente (grupo de dispositivos) ou por máquina.
// O app a recebe pelo heartbeat (mesmo canal da versão Pro) e aplica sozinho.
type NextecAppPolicy struct {
	Id         uint   `json:"id" gorm:"primaryKey"`
	Kind       string `json:"kind" gorm:"size:8;not null;uniqueIndex:idx_nextec_app_policy"` // group | peer
	Ref        string `json:"ref" gorm:"size:64;not null;uniqueIndex:idx_nextec_app_policy"` // id do grupo ou ID da máquina
	Enabled    bool   `json:"enabled"`
	Options    string `json:"options" gorm:"type:text"` // JSON {"enable-file-transfer":"N", ...}
	ModifiedAt int64  `json:"modified_at"`              // milissegundos; é o "modified_at" que o app guarda
}
