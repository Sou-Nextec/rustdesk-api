package global

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// NextecSettingsFile guarda as configurações alteradas pelo painel em tempo de execução.
// Fica no diretório data (volume do Docker), então sobrevive a reinícios e recriações do contêiner.
const NextecSettingsFile = "./data/nextec-settings.json"

var nextecSettingsMu sync.Mutex

// LoadNextecSettings é chamado na inicialização e sobrescreve o valor do config.yaml e das variáveis de ambiente.
func LoadNextecSettings() error {
	nextecSettingsMu.Lock()
	defer nextecSettingsMu.Unlock()
	s, err := readNextecSettings()
	if err != nil {
		return err
	}
	if v, ok := s["web_client"]; ok {
		var wc int
		if err := json.Unmarshal(v, &wc); err != nil {
			return err
		}
		Config.App.WebClient = wc
	}
	return nil
}

// SetWebClient liga ou desliga o cliente web e grava a escolha no arquivo.
func SetWebClient(enabled bool) error {
	nextecSettingsMu.Lock()
	defer nextecSettingsMu.Unlock()
	wc := 0
	if enabled {
		wc = 1
	}
	s, err := readNextecSettings()
	if err != nil {
		return err
	}
	s["web_client"], _ = json.Marshal(wc)
	if err := writeNextecSettings(s); err != nil {
		return err
	}
	Config.App.WebClient = wc
	return nil
}

// NextecClientTemplates devolve os modelos de cliente (lista JSON) guardados no servidor; vazio se não houver.
func NextecClientTemplates() (json.RawMessage, error) {
	nextecSettingsMu.Lock()
	defer nextecSettingsMu.Unlock()
	s, err := readNextecSettings()
	if err != nil {
		return nil, err
	}
	if v, ok := s["client_templates"]; ok {
		return v, nil
	}
	return json.RawMessage("[]"), nil
}

// SetNextecClientTemplates grava os modelos de cliente (lista JSON) no mesmo arquivo das demais configurações.
func SetNextecClientTemplates(v json.RawMessage) error {
	nextecSettingsMu.Lock()
	defer nextecSettingsMu.Unlock()
	s, err := readNextecSettings()
	if err != nil {
		return err
	}
	s["client_templates"] = v
	return writeNextecSettings(s)
}

func readNextecSettings() (map[string]json.RawMessage, error) {
	s := map[string]json.RawMessage{}
	b, err := os.ReadFile(NextecSettingsFile)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return s, nil
}

// writeNextecSettings grava num arquivo temporário e renomeia, para nunca deixar o arquivo pela metade.
func writeNextecSettings(s map[string]json.RawMessage) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(NextecSettingsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".nextec-settings-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), NextecSettingsFile)
}

// NextecSettingGet lê uma chave de texto das configurações da Nextec (vazio se não existir).
func NextecSettingGet(key string) string {
	nextecSettingsMu.Lock()
	defer nextecSettingsMu.Unlock()
	s, err := readNextecSettings()
	if err != nil {
		return ""
	}
	var v string
	if raw, ok := s[key]; ok {
		_ = json.Unmarshal(raw, &v)
	}
	return v
}

// NextecSettingSet grava uma chave de texto nas configurações da Nextec.
func NextecSettingSet(key, value string) error {
	nextecSettingsMu.Lock()
	defer nextecSettingsMu.Unlock()
	s, err := readNextecSettings()
	if err != nil {
		return err
	}
	s[key], _ = json.Marshal(value)
	return writeNextecSettings(s)
}
