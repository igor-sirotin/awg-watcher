package watch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const sampleAWGConfig = `[Interface]
PrivateKey = abcdefghijklmnop
Address = 10.0.0.2/32
Jc = 4

[Peer]
PublicKey = peer-public
PresharedKey = zyxwvutsrqponmlk
AllowedIPs = 0.0.0.0/0
Endpoint = 1.2.3.4:51820
`

func TestPreviewAWGConfigRedactsSecrets(t *testing.T) {
	preview, err := PreviewAWGConfig(sampleAWGConfig)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Interface["PrivateKey"] == "abcdefghijklmnop" {
		t.Fatalf("private key was not redacted")
	}
	if got := preview.Interface["Address"]; got != "10.0.0.2/32" {
		t.Fatalf("address = %q", got)
	}
	if len(preview.Peers) != 1 {
		t.Fatalf("peers = %d", len(preview.Peers))
	}
	if preview.Peers[0]["PresharedKey"] == "zyxwvutsrqponmlk" {
		t.Fatalf("preshared key was not redacted")
	}
}

func TestAWGManagerClientListExportReplace(t *testing.T) {
	var replaced bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			if r.Method != http.MethodPost {
				t.Fatalf("login method = %s", r.Method)
			}
			http.SetCookie(w, &http.Cookie{Name: "awg_session", Value: "session", Path: "/"})
			writeAWGEnvelope(t, w, map[string]any{"login": "admin"})
		case "/api/tunnels/list":
			requireAWGSession(t, r)
			writeAWGEnvelope(t, w, []map[string]any{{
				"id": "tun-1", "name": "Premium EE", "status": "running", "interfaceName": "awg0",
			}})
		case "/api/tunnels/export":
			requireAWGSession(t, r)
			if got := r.URL.Query().Get("id"); got != "tun-1" {
				t.Fatalf("export id = %q", got)
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(sampleAWGConfig))
		case "/api/tunnels/replace":
			requireAWGSession(t, r)
			if got := r.URL.Query().Get("id"); got != "tun-1" {
				t.Fatalf("replace id = %q", got)
			}
			var req struct {
				Content string `json:"content"`
				Name    string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Content != sampleAWGConfig {
				t.Fatalf("replace content mismatch")
			}
			replaced = true
			writeAWGEnvelope(t, w, map[string]any{"id": "tun-1", "name": req.Name})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := AWGManagerClient{Config: AWGManagerConfig{BaseURL: server.URL, Login: "admin", Password: "secret"}}
	tunnels, err := client.ListTunnels(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tunnels) != 1 || tunnels[0].ID != "tun-1" {
		t.Fatalf("unexpected tunnels: %+v", tunnels)
	}
	exported, err := client.ExportTunnel(context.Background(), "tun-1")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if string(exported) != sampleAWGConfig {
		t.Fatalf("exported content mismatch")
	}
	tunnel, _, err := client.ReplaceTunnel(context.Background(), "tun-1", sampleAWGConfig, "Premium NL")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !replaced || tunnel["name"] != "Premium NL" {
		t.Fatalf("replace did not return updated tunnel: %+v", tunnel)
	}
}

func TestReplaceAWGTunnelBacksUpBeforeReplace(t *testing.T) {
	var replaced bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tunnels/export":
			_, _ = w.Write([]byte("old config"))
		case "/api/tunnels/replace":
			replaced = true
			writeAWGEnvelope(t, w, map[string]any{"id": "tun-1"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.AWGManager = AWGManagerConfig{BaseURL: server.URL}
	app := NewApp(&Paths{
		ConfigPath:   filepath.Join(dir, "config.json"),
		StatePath:    filepath.Join(dir, "state.json"),
		AWGBackupDir: filepath.Join(dir, "backups"),
	}, cfg, "", "")

	result, err := app.ReplaceAWGTunnel(context.Background(), "tun-1", sampleAWGConfig, "")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !replaced {
		t.Fatalf("replace endpoint was not called")
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != "old config" {
		t.Fatalf("backup = %q", backup)
	}
}

func writeAWGEnvelope(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data}); err != nil {
		t.Fatal(err)
	}
}

func requireAWGSession(t *testing.T, r *http.Request) {
	t.Helper()
	if _, err := r.Cookie("awg_session"); err != nil {
		t.Fatalf("missing awg_session cookie")
	}
}
