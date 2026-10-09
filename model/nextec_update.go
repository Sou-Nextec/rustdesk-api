package model

// NextecAppRelease é uma versão do app (MSI/EXE gerado pelo rdgen) guardada no servidor para as máquinas se atualizarem.
type NextecAppRelease struct {
	Id         uint   `json:"id" gorm:"primaryKey"`
	Version    string `json:"version" gorm:"size:32;not null;uniqueIndex"`
	FileName   string `json:"file_name" gorm:"size:128;not null"`
	Sha256     string `json:"sha256" gorm:"size:64;not null"`
	Size       int64  `json:"size"`
	Notes      string `json:"notes" gorm:"size:1000"`
	UploadedBy string `json:"uploaded_by" gorm:"size:64"`
	CreatedAt  int64  `json:"created_at"`
}

// NextecAppInstall guarda o que cada máquina informou na última checagem de atualização.
type NextecAppInstall struct {
	PeerId  string `json:"peer_id" gorm:"primaryKey;size:64"`
	Version string `json:"version" gorm:"size:32"` // versão instalada informada pela máquina ("" = não informou)
	Target  string `json:"target" gorm:"size:32"`  // versão oferecida a ela na última checagem ("" = nenhuma)
	SeenAt  int64  `json:"seen_at"`
	Ip      string `json:"ip" gorm:"size:64"`
}
