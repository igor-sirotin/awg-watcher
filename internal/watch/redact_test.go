package watch

import "testing"

func TestRedactValue(t *testing.T) {
	v := map[string]any{
		"vpn_key": "vpn://abcdefghijklmnopqrstuvwxyz",
		"auth_data": map[string]any{
			"api_key": "supersecretvalue",
		},
		"telegram": map[string]any{
			"bot_token": "1234567890",
		},
		"config": "[Interface]\nPrivateKey = abc\n",
	}
	got := RedactValue(v).(map[string]any)
	if got["vpn_key"] == v["vpn_key"] {
		t.Fatal("vpn key was not redacted")
	}
	auth := got["auth_data"].(map[string]any)
	if auth["api_key"] == "supersecretvalue" {
		t.Fatal("api key was not redacted")
	}
	if got["config"] != "[redacted]" {
		t.Fatalf("native config not redacted: %v", got["config"])
	}
}
