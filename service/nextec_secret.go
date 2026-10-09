package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Cofre de senhas dos servidores (Nextec): cada máquina marcada recebe uma senha permanente do RustDesk
// que muda sozinha a cada N minutos. Um agente na máquina aplica a senha; quem tem acesso à máquina
// conecta pelo painel sem digitar nada; o admin pode ver a senha (com registro em auditoria).

const (
	NextecSecretDefaultInterval = 180  // minutos (3 horas)
	NextecSecretMinInterval     = 15   // minutos
	NextecSecretMaxInterval     = 1440 // minutos (24 horas)
	settingAgentKeyHash         = "agent_key_hash"
	vaultKeyFile                = "./data/nextec-vault.key"
)

var (
	ErrNextecBadAgentAuth   = errors.New("agent auth failed")
	ErrNextecAlreadyEnroll  = errors.New("already enrolled")
	ErrNextecBadPassword    = errors.New("invalid password format")
	ErrNextecNoPending      = errors.New("no pending password")
	ErrNextecTooManyFails   = errors.New("too many failures")
	nextecPasswordPattern   = regexp.MustCompile(`^[A-Za-z0-9]{16,64}$`)
	nextecVaultKeyOnce      sync.Once
	nextecVaultKeyValue     []byte
	nextecVaultKeyErr       error
	nextecFailMu            sync.Mutex
	nextecFailures          = map[string][]int64{}
	nextecFailureWindowSecs = int64(600)
	nextecFailureLimit      = 10
)

// ---------- criptografia ----------

// vaultKey: variável NEXTEC_VAULT_KEY (base64 de 32 bytes) ou arquivo gerado na primeira vez no volume de dados.
func vaultKey() ([]byte, error) {
	nextecVaultKeyOnce.Do(func() {
		if v := os.Getenv("NEXTEC_VAULT_KEY"); v != "" {
			k, err := base64.StdEncoding.DecodeString(v)
			if err != nil || len(k) != 32 {
				nextecVaultKeyErr = errors.New("NEXTEC_VAULT_KEY deve ser base64 de 32 bytes")
				return
			}
			nextecVaultKeyValue = k
			return
		}
		if b, err := os.ReadFile(vaultKeyFile); err == nil {
			k, err := base64.StdEncoding.DecodeString(string(trimNL(b)))
			if err == nil && len(k) == 32 {
				nextecVaultKeyValue = k
				return
			}
		}
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			nextecVaultKeyErr = err
			return
		}
		if err := os.MkdirAll("./data", 0755); err != nil {
			nextecVaultKeyErr = err
			return
		}
		if err := os.WriteFile(vaultKeyFile, []byte(base64.StdEncoding.EncodeToString(k)+"\n"), 0600); err != nil {
			nextecVaultKeyErr = err
			return
		}
		nextecVaultKeyValue = k
	})
	return nextecVaultKeyValue, nextecVaultKeyErr
}

func trimNL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

func vaultEncrypt(plain string) (string, error) {
	k, err := vaultKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func vaultDecrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	k, err := vaultKey()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("dados do cofre inválidos")
	}
	out, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- chave de cadastro dos agentes ----------

// NextecAgentKeyIsSet diz se já existe uma chave de cadastro.
func NextecAgentKeyIsSet() bool { return global.NextecSettingGet(settingAgentKeyHash) != "" }

// NextecAgentKeyRotate gera uma chave nova (devolvida só desta vez; no servidor fica apenas o hash).
func NextecAgentKeyRotate() (string, error) {
	key := randomHex(24)
	if err := global.NextecSettingSet(settingAgentKeyHash, sha256Hex(key)); err != nil {
		return "", err
	}
	return key, nil
}

func agentKeyOk(key string) bool {
	want := global.NextecSettingGet(settingAgentKeyHash)
	if want == "" || key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(sha256Hex(key)), []byte(want)) == 1
}

// ---------- limite de tentativas inválidas por IP ----------

func nextecFailCheck(ip string) bool {
	nextecFailMu.Lock()
	defer nextecFailMu.Unlock()
	now := time.Now().Unix()
	kept := nextecFailures[ip][:0]
	for _, t := range nextecFailures[ip] {
		if now-t < nextecFailureWindowSecs {
			kept = append(kept, t)
		}
	}
	nextecFailures[ip] = kept
	return len(kept) < nextecFailureLimit
}

func nextecFailAdd(ip string) {
	nextecFailMu.Lock()
	defer nextecFailMu.Unlock()
	nextecFailures[ip] = append(nextecFailures[ip], time.Now().Unix())
}

// ---------- regras ----------

// NextecSecretUpsertPolicy cria ou altera a regra de um grupo ou de uma máquina.
func NextecSecretUpsertPolicy(kind, ref string, enabled bool, interval int) (*model.NextecSecretPolicy, error) {
	if kind != "group" && kind != "peer" {
		return nil, errors.New("kind inválido")
	}
	if ref == "" {
		return nil, errors.New("ref inválido")
	}
	if interval == 0 {
		interval = NextecSecretDefaultInterval
	}
	if interval < NextecSecretMinInterval || interval > NextecSecretMaxInterval {
		return nil, errors.New("intervalo fora do limite")
	}
	p := &model.NextecSecretPolicy{}
	DB.Where("kind = ? and ref = ?", kind, ref).First(p)
	p.Kind, p.Ref, p.Enabled, p.IntervalMinutes = kind, ref, enabled, interval
	if err := DB.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func NextecSecretDeletePolicy(kind, ref string) error {
	return DB.Where("kind = ? and ref = ?", kind, ref).Delete(&model.NextecSecretPolicy{}).Error
}

func NextecSecretPolicies() (res []*model.NextecSecretPolicy) {
	DB.Order("kind, ref").Find(&res)
	return res
}

// effectivePolicy: a regra da máquina vence a do grupo.
func effectivePolicy(peerID string, groupID uint) (bool, int) {
	p := &model.NextecSecretPolicy{}
	if DB.Where("kind = ? and ref = ?", "peer", peerID).First(p).Error == nil {
		return p.Enabled, p.IntervalMinutes
	}
	if groupID != 0 && DB.Where("kind = ? and ref = ?", "group", strconv.Itoa(int(groupID))).First(p).Error == nil {
		return p.Enabled, p.IntervalMinutes
	}
	return false, 0
}

func peerGroup(peerID string) uint {
	pr := &model.Peer{}
	DB.Select("group_id").Where("id = ?", peerID).First(pr)
	return pr.GroupId
}

// ---------- auditoria ----------

func NextecSecretAuditAdd(u *model.User, peerID, action, ip, detail string) {
	a := &model.NextecSecretAudit{At: time.Now().Unix(), PeerId: peerID, Action: action, Ip: ip, Detail: detail}
	if u != nil {
		a.UserId, a.Username = u.Id, u.Username
	}
	if err := DB.Create(a).Error; err != nil {
		Logger.Warn("nextec audit: ", err)
	}
}

func NextecSecretAuditList(limit int) (res []*model.NextecSecretAudit) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	DB.Order("id desc").Limit(limit).Find(&res)
	return res
}

// ---------- agente ----------

func agentAuth(ip, peerID, token string) (*model.NextecDeviceSecret, error) {
	if !nextecFailCheck(ip) {
		return nil, ErrNextecTooManyFails
	}
	d := &model.NextecDeviceSecret{}
	if peerID == "" || token == "" || DB.Where("peer_id = ?", peerID).First(d).Error != nil || d.TokenHash == "" ||
		subtle.ConstantTimeCompare([]byte(sha256Hex(token)), []byte(d.TokenHash)) != 1 {
		nextecFailAdd(ip)
		return nil, ErrNextecBadAgentAuth
	}
	return d, nil
}

// NextecAgentEnroll cadastra a máquina e devolve o token dela. Máquina já cadastrada só volta a cadastrar depois de o admin remover o agente.
func NextecAgentEnroll(ip, key, peerID string) (string, error) {
	if !nextecFailCheck(ip) {
		return "", ErrNextecTooManyFails
	}
	if !agentKeyOk(key) || peerID == "" || len(peerID) > 32 {
		nextecFailAdd(ip)
		return "", ErrNextecBadAgentAuth
	}
	d := &model.NextecDeviceSecret{}
	found := DB.Where("peer_id = ?", peerID).First(d).Error == nil
	if found && d.TokenHash != "" {
		nextecFailAdd(ip)
		return "", ErrNextecAlreadyEnroll
	}
	token := randomHex(32)
	d.PeerId, d.TokenHash = peerID, sha256Hex(token)
	d.LastSeenAt = time.Now().Unix()
	if err := DB.Save(d).Error; err != nil {
		return "", err
	}
	NextecSecretAuditAdd(nil, peerID, "agent_enroll", ip, "")
	return token, nil
}

type NextecAgentSync struct {
	Managed  bool `json:"managed"`
	Rotate   bool `json:"rotate"`
	Interval int  `json:"interval_minutes"`
}

// NextecAgentSyncState diz ao agente se a máquina é gerenciada e se está na hora de trocar a senha.
func NextecAgentSyncState(ip, peerID, token string) (*NextecAgentSync, error) {
	d, err := agentAuth(ip, peerID, token)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	DB.Model(d).Update("last_seen_at", now)
	enabled, interval := effectivePolicy(peerID, peerGroup(peerID))
	if !enabled {
		return &NextecAgentSync{}, nil
	}
	rotate := d.ActiveEnc == "" || d.ForceRotate || now >= d.NextRotationAt
	return &NextecAgentSync{Managed: true, Rotate: rotate, Interval: interval}, nil
}

// NextecAgentPropose guarda a senha nova como pendente (a antiga continua valendo até o agente confirmar).
func NextecAgentPropose(ip, peerID, token, password string) error {
	d, err := agentAuth(ip, peerID, token)
	if err != nil {
		return err
	}
	if !nextecPasswordPattern.MatchString(password) {
		return ErrNextecBadPassword
	}
	enc, err := vaultEncrypt(password)
	if err != nil {
		return err
	}
	return DB.Model(d).Update("pending_enc", enc).Error
}

// NextecAgentConfirm promove a senha pendente a ativa, depois de o agente aplicá-la na máquina.
func NextecAgentConfirm(ip, peerID, token string) error {
	d, err := agentAuth(ip, peerID, token)
	if err != nil {
		return err
	}
	if d.PendingEnc == "" {
		return ErrNextecNoPending
	}
	_, interval := effectivePolicy(peerID, peerGroup(peerID))
	if interval == 0 {
		interval = NextecSecretDefaultInterval
	}
	now := time.Now().Unix()
	err = DB.Model(d).Updates(map[string]interface{}{
		"active_enc": d.PendingEnc, "pending_enc": "", "version": d.Version + 1,
		"active_at": now, "next_rotation_at": now + int64(interval)*60, "force_rotate": false,
	}).Error
	if err == nil {
		NextecSecretAuditAdd(nil, peerID, "rotated", ip, "versão "+strconv.Itoa(d.Version+1))
	}
	return err
}

// ---------- painel (admin) ----------

type NextecSecretDevice struct {
	PeerId         string `json:"peer_id"`
	Hostname       string `json:"hostname"`
	Alias          string `json:"alias"`
	GroupId        uint   `json:"group_id"`
	Managed        bool   `json:"managed"`
	IntervalMin    int    `json:"interval_minutes"`
	Enrolled       bool   `json:"enrolled"`
	HasPassword    bool   `json:"has_password"`
	Pending        bool   `json:"pending"`
	Version        int    `json:"version"`
	ActiveAt       int64  `json:"active_at"`
	NextRotationAt int64  `json:"next_rotation_at"`
	LastSeenAt     int64  `json:"last_seen_at"`
	ForceRotate    bool   `json:"force_rotate"`
}

// NextecSecretDevices lista as máquinas gerenciadas ou com agente cadastrado.
func NextecSecretDevices() []*NextecSecretDevice {
	var states []*model.NextecDeviceSecret
	DB.Find(&states)
	byID := map[string]*model.NextecDeviceSecret{}
	for _, s := range states {
		byID[s.PeerId] = s
	}
	policies := NextecSecretPolicies()
	ids := map[string]bool{}
	groupIDs := []uint{}
	for _, s := range states {
		ids[s.PeerId] = true
	}
	for _, p := range policies {
		if p.Kind == "peer" {
			ids[p.Ref] = true
		} else if g, err := strconv.Atoi(p.Ref); err == nil && p.Enabled {
			groupIDs = append(groupIDs, uint(g))
		}
	}
	var peers []*model.Peer
	q := DB.Model(&model.Peer{})
	idList := make([]string, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	switch {
	case len(idList) > 0 && len(groupIDs) > 0:
		q = q.Where("id in ? or group_id in ?", idList, groupIDs)
	case len(idList) > 0:
		q = q.Where("id in ?", idList)
	case len(groupIDs) > 0:
		q = q.Where("group_id in ?", groupIDs)
	default:
		return []*NextecSecretDevice{}
	}
	q.Find(&peers)
	seen := map[string]bool{}
	out := make([]*NextecSecretDevice, 0, len(peers)+len(states))
	add := func(peerID, hostname, alias string, groupID uint) {
		if seen[peerID] {
			return
		}
		seen[peerID] = true
		enabled, interval := effectivePolicy(peerID, groupID)
		dv := &NextecSecretDevice{PeerId: peerID, Hostname: hostname, Alias: alias, GroupId: groupID, Managed: enabled, IntervalMin: interval}
		if s := byID[peerID]; s != nil {
			dv.Enrolled = s.TokenHash != ""
			dv.HasPassword = s.ActiveEnc != ""
			dv.Pending = s.PendingEnc != ""
			dv.Version, dv.ActiveAt, dv.NextRotationAt = s.Version, s.ActiveAt, s.NextRotationAt
			dv.LastSeenAt, dv.ForceRotate = s.LastSeenAt, s.ForceRotate
		}
		out = append(out, dv)
	}
	for _, p := range peers {
		add(p.Id, p.Hostname, p.Alias, p.GroupId)
	}
	for _, s := range states { // agentes de máquinas que ainda não aparecem em Dispositivos
		add(s.PeerId, "", "", 0)
	}
	return out
}

// NextecSecretRotateNow manda trocar a senha na próxima verificação do agente.
func NextecSecretRotateNow(peerID string) error {
	res := DB.Model(&model.NextecDeviceSecret{}).Where("peer_id = ?", peerID).Update("force_rotate", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("a máquina ainda não tem agente cadastrado")
	}
	return nil
}

// NextecSecretUnenroll remove o agente (a máquina precisa se cadastrar de novo) e apaga as senhas guardadas.
func NextecSecretUnenroll(peerID string) error {
	return DB.Where("peer_id = ?", peerID).Delete(&model.NextecDeviceSecret{}).Error
}

type NextecRevealed struct {
	Password string `json:"password"`
	Pending  string `json:"pending,omitempty"`
	Version  int    `json:"version"`
	ActiveAt int64  `json:"active_at"`
}

// NextecSecretReveal devolve a senha vigente (e a pendente, se houver) em texto, para o admin.
func NextecSecretReveal(peerID string) (*NextecRevealed, error) {
	d := &model.NextecDeviceSecret{}
	if err := DB.Where("peer_id = ?", peerID).First(d).Error; err != nil {
		return nil, errors.New("máquina sem senha guardada")
	}
	active, err := vaultDecrypt(d.ActiveEnc)
	if err != nil {
		return nil, err
	}
	pending, err := vaultDecrypt(d.PendingEnc)
	if err != nil {
		return nil, err
	}
	if active == "" && pending == "" {
		return nil, errors.New("a máquina ainda não tem senha")
	}
	return &NextecRevealed{Password: active, Pending: pending, Version: d.Version, ActiveAt: d.ActiveAt}, nil
}

// ---------- conectar pelo painel ----------

// NextecUserSeesPeer diz se o usuário pode conectar na máquina: admin, máquina dele, lista dele ou lista compartilhada com ele.
func NextecUserSeesPeer(u *model.User, peerID string) bool {
	if u == nil || peerID == "" {
		return false
	}
	if AllService.UserService.IsAdmin(u) {
		return true
	}
	var n int64
	DB.Model(&model.Peer{}).Where("id = ? and user_id = ?", peerID, u.Id).Count(&n)
	if n > 0 {
		return true
	}
	DB.Model(&model.AddressBook{}).Where("id = ? and user_id = ?", peerID, u.Id).Count(&n)
	if n > 0 {
		return true
	}
	cols := map[uint]bool{}
	for _, r := range AllService.AddressBookService.CollectionReadRules(u) {
		cols[r.CollectionId] = true
	}
	if len(cols) == 0 {
		return false
	}
	ids := make([]uint, 0, len(cols))
	for id := range cols {
		ids = append(ids, id)
	}
	DB.Model(&model.AddressBook{}).Where("id = ? and collection_id in ?", peerID, ids).Count(&n)
	return n > 0
}

// NextecConnectPassword devolve a senha vigente de uma máquina gerenciada para quem pode conectar nela ("" se não houver).
func NextecConnectPassword(u *model.User, peerID string) (string, bool) {
	if !NextecUserSeesPeer(u, peerID) {
		return "", false
	}
	enabled, _ := effectivePolicy(peerID, peerGroup(peerID))
	if !enabled {
		return "", true
	}
	d := &model.NextecDeviceSecret{}
	if DB.Where("peer_id = ?", peerID).First(d).Error != nil || d.ActiveEnc == "" {
		return "", true
	}
	pw, err := vaultDecrypt(d.ActiveEnc)
	if err != nil {
		Logger.Warn("nextec vault decrypt: ", err)
		return "", true
	}
	return pw, true
}
