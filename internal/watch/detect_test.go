package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectBaselineAndChange(t *testing.T) {
	b, err := os.ReadFile("../../testdata/account_info_baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := ParseAccountInfo(b)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Countries: []string{"EE", "NL"}}
	st := &State{Countries: map[string]CountryState{}}
	result := DetectChanges(cfg, st, info)
	if result.Status != "ok" {
		t.Fatalf("baseline status = %s", result.Status)
	}
	if st.Countries["EE"].Status != "baseline" {
		t.Fatalf("expected EE baseline, got %+v", st.Countries["EE"])
	}

	b, err = os.ReadFile("../../testdata/account_info_changed.json")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := ParseAccountInfo(b)
	if err != nil {
		t.Fatal(err)
	}
	result = DetectChanges(cfg, st, changed)
	if result.Status != "changed" {
		t.Fatalf("change status = %s", result.Status)
	}
	if st.Countries["EE"].Status != "changed" {
		t.Fatalf("expected EE changed, got %+v", st.Countries["EE"])
	}
	if st.Countries["NL"].Status != "ok" {
		t.Fatalf("expected NL ok, got %+v", st.Countries["NL"])
	}
}

func TestDetectMissingCountry(t *testing.T) {
	b, err := os.ReadFile("../../testdata/account_info_baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := ParseAccountInfo(b)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Countries: []string{"US"}}
	st := &State{Countries: map[string]CountryState{}}
	result := DetectChanges(cfg, st, info)
	if result.Status != "changed" {
		t.Fatalf("missing status = %s", result.Status)
	}
	if st.Countries["US"].Status != "missing" {
		t.Fatalf("expected US missing, got %+v", st.Countries["US"])
	}
}

func TestAppCheckStoresPerKeyState(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Keys = []KeyConfig{
		{ID: "one", Name: "One", VPNKey: "fixture", Countries: []string{"EE"}},
		{ID: "two", Name: "Two", VPNKey: "fixture", Countries: []string{"NL"}},
	}
	paths := &Paths{
		ConfigPath:           filepath.Join(dir, "config.json"),
		StatePath:            filepath.Join(dir, "state.json"),
		GatewayPublicKeyPath: filepath.Join(dir, "gateway_public_key.pem"),
	}
	app := NewApp(paths, cfg, "../../testdata/account_info_baseline.json", "")
	result, err := app.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %s", result.Status)
	}
	state, err := LoadState(paths.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.Keys["one"].Countries["EE"].Status != "baseline" {
		t.Fatalf("key one state = %+v", state.Keys["one"])
	}
	if state.Keys["two"].Countries["NL"].Status != "baseline" {
		t.Fatalf("key two state = %+v", state.Keys["two"])
	}
}

func TestApplyDefaultsMigratesSingleKeyConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VPNKey = "vpn://old"
	cfg.Countries = []string{"nl", "EE"}
	applyDefaults(cfg)
	if len(cfg.Keys) != 1 {
		t.Fatalf("keys = %+v", cfg.Keys)
	}
	if cfg.Keys[0].ID != "default" || cfg.Keys[0].VPNKey != "vpn://old" {
		t.Fatalf("migrated key = %+v", cfg.Keys[0])
	}
	if cfg.Keys[0].Countries[0] != "EE" || cfg.Keys[0].Countries[1] != "NL" {
		t.Fatalf("countries = %+v", cfg.Keys[0].Countries)
	}
}
