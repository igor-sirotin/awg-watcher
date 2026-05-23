package watch

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestDecodeVPNKeyPlainAndCompressed(t *testing.T) {
	payload := map[string]any{
		"api_config": map[string]any{
			"service_type":      "amnezia-premium",
			"service_protocol":  "awg",
			"user_country_code": "EE",
		},
		"auth_data": map[string]any{"api_key": "secret"},
	}
	b, _ := json.Marshal(payload)
	plain := "vpn://" + base64.RawURLEncoding.EncodeToString(b)
	for _, key := range []string{plain, compressedKey(b)} {
		decoded, err := DecodeVPNKey(key)
		if err != nil {
			t.Fatal(err)
		}
		auth, err := ExtractPremiumAuth(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if auth.ServiceType != "amnezia-premium" || auth.APIKey != "secret" {
			t.Fatalf("unexpected auth: %+v", auth)
		}
	}
}

func compressedKey(payload []byte) string {
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write(payload)
	_ = zw.Close()
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, uint32(len(payload)))
	out.Write(z.Bytes())
	return "vpn://" + base64.RawURLEncoding.EncodeToString(out.Bytes())
}
