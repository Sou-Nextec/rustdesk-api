package model

// NextecSecretPolicy liga a senha automática para um grupo de dispositivos (Kind "group", Ref = id do grupo)
// ou para uma máquina (Kind "peer", Ref = ID do RustDesk). A regra da máquina vale mais que a do grupo.
type NextecSecretPolicy struct {
	Id              uint   `gorm:"primaryKey" json:"id"`
	Kind            string `gorm:"not null;index:idx_nx_policy,unique" json:"kind"`
	Ref             string `gorm:"not null;index:idx_nx_policy,unique" json:"ref"`
	Enabled         bool   `gorm:"not null;default:true" json:"enabled"`
	IntervalMinutes int    `gorm:"not null;default:180" json:"interval_minutes"`
	TimeModel
}

// NextecDeviceSecret é o estado da senha de uma máquina. As senhas ficam criptografadas (AES-GCM).
type NextecDeviceSecret struct {
	Id             uint   `gorm:"primaryKey" json:"id"`
	PeerId         string `gorm:"not null;uniqueIndex" json:"peer_id"`
	TokenHash      string `gorm:"not null;default:''" json:"-"`
	ActiveEnc      string `gorm:"not null;default:''" json:"-"`
	PendingEnc     string `gorm:"not null;default:''" json:"-"`
	Version        int    `gorm:"not null;default:0" json:"version"`
	ActiveAt       int64  `gorm:"not null;default:0" json:"active_at"`
	NextRotationAt int64  `gorm:"not null;default:0" json:"next_rotation_at"`
	ForceRotate    bool   `gorm:"not null;default:false" json:"force_rotate"`
	LastSeenAt     int64  `gorm:"not null;default:0" json:"last_seen_at"`
	TimeModel
}

// NextecSecretAudit registra quem viu, usou ou trocou senhas.
type NextecSecretAudit struct {
	Id       uint   `gorm:"primaryKey" json:"id"`
	At       int64  `gorm:"not null;index" json:"at"`
	UserId   uint   `gorm:"not null;default:0" json:"user_id"`
	Username string `gorm:"not null;default:''" json:"username"`
	PeerId   string `gorm:"not null;default:'';index" json:"peer_id"`
	Action   string `gorm:"not null;default:''" json:"action"`
	Ip       string `gorm:"not null;default:''" json:"ip"`
	Detail   string `gorm:"not null;default:''" json:"detail"`
}
