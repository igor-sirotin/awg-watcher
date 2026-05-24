package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	key := "AMNEZIA_CONFIG_WATCH_TEST_PUBLIC_KEY"
	_ = os.Unsetenv(key)
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	path := filepath.Join(t.TempDir(), ".env")
	err := os.WriteFile(path, []byte(key+`="-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	got := os.Getenv(key)
	want := "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"
	if got != want {
		t.Fatalf("env value mismatch: %q", got)
	}
}
