package watch

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
	got, err := gatewayPublicKey(&Config{Amnezia: AmneziaConfig{GatewayPublicKeyFilePath: path}})
	if err != nil {
		t.Fatal(err)
	}
	if got.N.Cmp(privateKey.PublicKey.N) != 0 || got.E != privateKey.PublicKey.E {
		t.Fatal("loaded public key does not match file")
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
