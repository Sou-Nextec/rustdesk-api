package service

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

var ErrNextecReleaseSignature = errors.New("A atualizaÃ§Ã£o precisa de um manifesto Connect vÃ¡lido, assinado pela Nextec, correspondente ao MSI enviado.")

// Public trust anchor shared with Nextec monitoring. No private key is on this server.
const nextecMonitoringRSAModulus = "d87a26b54c0a3ca1d1780b5d72b9f81b7bcfcaa37c2b6ac57619e95aed2d6441da58de1133acb8cf7ada60a9d2e5506662b78aac1e8b77e5d170cb4066ae799cddb20dbbfe14919e92a350cbc01b2045d52058bd4975a880d65b8cb9aff4932e7ce1aedb64fe049f2c889105c6086f023d9a30788d523d1b551936a413db547bd53e391ef1a81fa80664f17516addf8ae23ab609b9f4e802baebf8777f6138ec4ff61c951dbbaabaa15fb85469642ff531b49c1a588b6b4870da9f6034c0aeb005a9fe8fa67d9efa7cad26e0057c272c4d6a7cebefc3f5c58bbd8d8c6553ab113c882ebe8d5b68b2a35c4b224b8dc7aa7a920b1b34a332d4763c5e51c4918b19d4894793b8be26ad4ae4d84afee77196692d6a26b44ccb6001e133d43755c5bf2b46e51b3f0b39c5ba4ce4eafbf4967d01b3776bdb8c5b4bc113a3c4ce0d5b12108b2c63225085eed92254b377c53baca735574b50af0a1a1a1cdd3f825b6e74d0a3461d7d4f45c4bb5b29b79631536d2af39ce02bc20b24e72054de5b38678389db8fee83fd48a3e0c6d7bd4caf33f6756c5a7a57293ef1694d8589e0a396b12881438b7933fbf41a7724ad96ebacc65fbd43f80bbe9c9d56a4637552e9cd5b73b7e8cf9be0db09701b5c26d1427dd92b508496e3fddd27013f080d015e712f7139cfd22ec3adcfc0776693a0649e3b1d4de08cd47340502c19b8d77460738b"

type NextecSignedRelease struct {
	Manifest  string `json:"manifesto"`
	Signature string `json:"assinatura"`
}

type nextecSignedManifest struct {
	Product  string    `json:"produto"`
	Format   int       `json:"formato"`
	Sequence int64     `json:"sequencia"`
	Version  string    `json:"versao"`
	Issued   time.Time `json:"publicado_em"`
	Expires  time.Time `json:"valido_ate"`
	Windows  *nextecSignedAsset `json:"windows"`
	Linux map[string]nextecSignedAsset `json:"linux"`
}

type nextecSignedAsset struct {
		File string `json:"arquivo"`
		Hash string `json:"sha256"`
		Size int64  `json:"tamanho"`
}

func nextecVerifySignedRelease(r *model.NextecAppRelease, envelope NextecSignedRelease, key *rsa.PublicKey, now time.Time) error {
	if r == nil || len(envelope.Manifest) > 88000 || len(envelope.Signature) > 4096 {
		return ErrNextecReleaseSignature
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.Manifest)
	if err != nil || len(raw) == 0 || len(raw) > 65536 {
		return ErrNextecReleaseSignature
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		return ErrNextecReleaseSignature
	}
	digest := sha256.Sum256(raw)
	if key.N.BitLen() < 3072 || rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return ErrNextecReleaseSignature
	}
	var m nextecSignedManifest
	if json.Unmarshal(raw, &m) != nil || m.Product != "nextec-connect" || m.Format != 1 || m.Sequence < 1 || m.Sequence > 9007199254740991 {
		return ErrNextecReleaseSignature
	}
	if m.Issued.After(now.Add(5*time.Minute)) || !m.Expires.After(now) || !m.Expires.After(m.Issued) || m.Expires.Sub(m.Issued) > 30*24*time.Hour {
		return ErrNextecReleaseSignature
	}
	if m.Version != r.Version {
		return ErrNextecReleaseSignature
	}
	assets := 0
	if r.FileName != "" {
		if m.Windows == nil || m.Windows.File != "files/"+r.FileName || m.Windows.Hash != r.Sha256 || m.Windows.Size != r.Size || r.FileName != "nextec-acesso-"+r.Version+".msi" {
			return ErrNextecReleaseSignature
		}
		assets++
	} else if m.Windows != nil {
		return ErrNextecReleaseSignature
	}
	for _, item := range []struct{ arch, name, hash string; size int64 }{
		{"amd64", r.LinuxAMD64FileName, r.LinuxAMD64Sha256, r.LinuxAMD64Size},
		{"arm64", r.LinuxARM64FileName, r.LinuxARM64Sha256, r.LinuxARM64Size},
	} {
		signed, ok := m.Linux[item.arch]
		if item.name == "" {
			if ok { return ErrNextecReleaseSignature }
			continue
		}
		if !ok || item.name != "nextec-acesso-"+r.Version+"-"+item.arch+".deb" || signed.File != "files/"+item.name || signed.Hash != item.hash || signed.Size != item.size || len(item.hash) != 64 || item.size < 8 || item.size > 300<<20 {
			return ErrNextecReleaseSignature
		}
		assets++
	}
	for arch := range m.Linux {
		if arch != "amd64" && arch != "arm64" {
			return ErrNextecReleaseSignature
		}
	}
	if len(m.Linux) > 2 || assets == 0 {
		return ErrNextecReleaseSignature
	}
	return nil
}

func nextecReleaseTrustKey() *rsa.PublicKey {
	n, _ := new(big.Int).SetString(nextecMonitoringRSAModulus, 16)
	return &rsa.PublicKey{N: n, E: 65537}
}

func NextecReleaseSignatureSave(version string, envelope NextecSignedRelease) error {
	r := NextecReleaseByVersion(version)
	if err := nextecVerifySignedRelease(r, envelope, nextecReleaseTrustKey(), time.Now()); err != nil {
		return err
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return global.NextecSettingSet("app_signature_"+r.Version, string(encoded))
}

func NextecReleaseSignature(r *model.NextecAppRelease) *NextecSignedRelease {
	if r == nil {
		return nil
	}
	var envelope NextecSignedRelease
	if json.Unmarshal([]byte(global.NextecSettingGet("app_signature_"+r.Version)), &envelope) != nil {
		return nil
	}
	if nextecVerifySignedRelease(r, envelope, nextecReleaseTrustKey(), time.Now()) != nil {
		return nil
	}
	return &envelope
}

