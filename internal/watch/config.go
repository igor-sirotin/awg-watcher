package watch

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	return cfg, nil
}

func DefaultConfig() *Config {
	cfg := &Config{
		ListenAddr:        DefaultListenAddr,
		PollIntervalHours: defaultPollIntervalHours,
		Amnezia: AmneziaConfig{
			GatewayEndpoint: DefaultGatewayEndpoint,
		},
		Telegram: TelegramConfig{Endpoint: DefaultTelegramEndpoint},
	}
	return cfg
}

func SaveConfig(path string, cfg *Config) error {
	applyDefaults(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

func LoadState(path string) (*State, error) {
	st := &State{Status: "unknown", Countries: map[string]CountryState{}, Keys: map[string]KeyState{}, LastNotified: map[string]string{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, err
	}
	if st.Countries == nil {
		st.Countries = map[string]CountryState{}
	}
	if st.Keys == nil {
		st.Keys = map[string]KeyState{}
	}
	if st.LastNotified == nil {
		st.LastNotified = map[string]string{}
	}
	if st.Status == "" {
		st.Status = "unknown"
	}
	return st, nil
}

func SaveState(path string, st *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

func GenerateSetupToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func GenerateKeyID() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "key-" + base64.RawURLEncoding.EncodeToString(b), nil
}

func applyDefaults(cfg *Config) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = DefaultListenAddr
	}
	if cfg.PollIntervalHours <= 0 {
		cfg.PollIntervalHours = defaultPollIntervalHours
	}
	if cfg.Amnezia.GatewayEndpoint == "" {
		cfg.Amnezia.GatewayEndpoint = DefaultGatewayEndpoint
	}
	if cfg.Telegram.Endpoint == "" {
		cfg.Telegram.Endpoint = DefaultTelegramEndpoint
	}
	migrateSingleKeyConfig(cfg)
	for i := range cfg.Keys {
		cfg.Keys[i].ID = strings.TrimSpace(cfg.Keys[i].ID)
		if cfg.Keys[i].ID == "" {
			cfg.Keys[i].ID = fmt.Sprintf("key-%d", i+1)
		}
		cfg.Keys[i].Name = strings.TrimSpace(cfg.Keys[i].Name)
		if cfg.Keys[i].Name == "" {
			cfg.Keys[i].Name = fmt.Sprintf("Key %d", i+1)
		}
		cfg.Keys[i].Countries = normalizeCountries(cfg.Keys[i].Countries)
	}
}

func migrateSingleKeyConfig(cfg *Config) {
	if strings.TrimSpace(cfg.VPNKey) == "" || len(cfg.Keys) > 0 {
		return
	}
	cfg.Keys = []KeyConfig{{
		ID:        "default",
		Name:      "Default key",
		VPNKey:    cfg.VPNKey,
		Countries: cfg.Countries,
	}}
}
