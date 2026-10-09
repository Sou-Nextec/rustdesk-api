package model

// NextecPendingAssign guarda o cliente escolhido na instalação enquanto a máquina ainda não se registrou no servidor.
type NextecPendingAssign struct {
	PeerId    string `json:"peer_id" gorm:"primaryKey;size:64"`
	GroupId   uint   `json:"group_id"`
	ExpiresAt int64  `json:"expires_at"`
}
