package watch

import (
	"os"
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
