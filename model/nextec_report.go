package model

// NextecConnectNote guarda o chamado informado por quem clicou em Conectar no painel. O relatório cruza com a auditoria
// de conexões (mesma máquina, poucos segundos antes de a conexão abrir).
type NextecConnectNote struct {
	Id       uint   `json:"id" gorm:"primaryKey"`
	UserId   uint   `json:"user_id"`
	Username string `json:"username" gorm:"size:64"`
	PeerId   string `json:"peer_id" gorm:"size:64;index:idx_nextec_note_peer_at"`
	Ticket   string `json:"ticket" gorm:"size:32"`
	Note     string `json:"note" gorm:"size:200"`
	At       int64  `json:"at" gorm:"index:idx_nextec_note_peer_at"`
}
