package watch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayPublicKeyFromFile(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gateway_public_key.pem")
	err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), 0600)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := gatewayPublicKeys(&Config{Amnezia: AmneziaConfig{GatewayPublicKeyFilePath: path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	got := keys[0]
	if got.N.Cmp(privateKey.PublicKey.N) != 0 || got.E != privateKey.PublicKey.E {
		t.Fatal("loaded public key does not match file")
	}
}

func TestGatewayPublicKeysFromFile(t *testing.T) {
	first := publicKeyPEM(t)
	second := publicKeyPEM(t)
	path := filepath.Join(t.TempDir(), "gateway_public_key.pem")
	err := os.WriteFile(path, append(first, second...), 0600)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := gatewayPublicKeys(&Config{Amnezia: AmneziaConfig{GatewayPublicKeyFilePath: path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestFetchAccountInfoTriesNextKeyAfterServerError(t *testing.T) {
	firstPrivate, firstPEM := generatedPublicKeyPEM(t)
	_ = firstPrivate
	secondPrivate, secondPEM := generatedPublicKeyPEM(t)

	keyPath := filepath.Join(t.TempDir(), "gateway_public_key.pem")
	if err := os.WriteFile(keyPath, append(firstPEM, secondPEM...), 0600); err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			http.Error(w, "wrong key", http.StatusInternalServerError)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		encryptedKeyPayload, err := base64.StdEncoding.DecodeString(req["key_payload"])
		if err != nil {
			t.Fatal(err)
		}
		keyPayloadJSON, err := rsa.DecryptPKCS1v15(rand.Reader, secondPrivate, encryptedKeyPayload)
		if err != nil {
			t.Fatal(err)
		}
		var keyPayload map[string]string
		if err := json.Unmarshal(keyPayloadJSON, &keyPayload); err != nil {
			t.Fatal(err)
		}
		aesKey, err := base64.StdEncoding.DecodeString(keyPayload["aes_key"])
		if err != nil {
			t.Fatal(err)
		}
		iv, err := base64.StdEncoding.DecodeString(keyPayload["aes_iv"])
		if err != nil {
			t.Fatal(err)
		}
		response, err := aesCBCEncrypt([]byte(`{"available_countries":[{"code":"EE"}],"issued_configs":[]}`), aesKey, iv[:16])
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	}))
	defer server.Close()

	info, err := (AccountClient{
		Config: &Config{
			VPNKey: testVPNKey(t),
			Amnezia: AmneziaConfig{
				GatewayEndpoint:          server.URL,
				GatewayPublicKeyFilePath: keyPath,
			},
		},
		HTTPClient: server.Client(),
	}).FetchAccountInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d", requestCount)
	}
	if len(info.AvailableCountries) != 1 || info.AvailableCountries[0].Code != "EE" {
		t.Fatalf("unexpected account info: %+v", info)
	}
}

func TestApplyWorkdirSetsGatewayPublicKeyPath(t *testing.T) {
	paths := DefaultPaths()
	paths.ApplyWorkdir("/tmp/amnezia-watch")
	if paths.GatewayPublicKeyPath != "/tmp/amnezia-watch/gateway_public_key.pem" {
		t.Fatalf("gateway key path = %s", paths.GatewayPublicKeyPath)
	}

	paths = DefaultPaths()
	paths.GatewayPublicKeyPath = "/private/key.pem"
	paths.ApplyWorkdir("/tmp/amnezia-watch")
	if paths.GatewayPublicKeyPath != "/private/key.pem" {
		t.Fatalf("explicit gateway key path was overwritten: %s", paths.GatewayPublicKeyPath)
	}
}

func publicKeyPEM(t *testing.T) []byte {
	t.Helper()
	_, pemBytes := generatedPublicKeyPEM(t)
	return pemBytes
}

func generatedPublicKeyPEM(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return privateKey, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub})
}

func testVPNKey(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"api_config": map[string]any{
			"service_type":      "amnezia-premium",
			"service_protocol":  "awg",
			"user_country_code": "EE",
		},
		"auth_data": map[string]any{"api_key": "fixture"},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return "vpn://" + base64.RawURLEncoding.EncodeToString(b)
}
