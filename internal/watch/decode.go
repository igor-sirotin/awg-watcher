package watch

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type PremiumAuth struct {
	ServiceType     string
	ServiceProtocol string
	UserCountryCode string
	APIKey          string
	AuthData        map[string]any
}

func DecodeVPNKey(key string) (map[string]any, error) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "vpn://"))
	key = strings.TrimPrefix(key, "vpn://")
	if key == "" {
		return nil, fmt.Errorf("empty vpn key")
	}

	var raw []byte
	var err error
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		raw, err = enc.DecodeString(key)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("base64 decode vpn key: %w", err)
	}

	payload := raw
	if len(raw) > 4 {
		declared := int(binary.BigEndian.Uint32(raw[:4]))
		if declared > 0 {
			if zr, zerr := zlib.NewReader(bytes.NewReader(raw[4:])); zerr == nil {
				inflated, rerr := io.ReadAll(zr)
				zr.Close()
				if rerr != nil {
					return nil, rerr
				}
				payload = inflated
			}
		}
	}

	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("decode vpn JSON: %w", err)
	}
	return obj, nil
}

func ExtractPremiumAuth(decoded map[string]any) (*PremiumAuth, error) {
	apiConfig, _ := decoded["api_config"].(map[string]any)
	authData, _ := decoded["auth_data"].(map[string]any)
	if apiConfig == nil || authData == nil {
		return nil, fmt.Errorf("vpn key does not contain api_config/auth_data")
	}
	apiKey, _ := authData["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("vpn key auth_data.api_key is empty")
	}
	auth := &PremiumAuth{
		ServiceType:     stringField(apiConfig, "service_type"),
		ServiceProtocol: stringField(apiConfig, "service_protocol"),
		UserCountryCode: stringField(apiConfig, "user_country_code"),
		APIKey:          apiKey,
		AuthData:        authData,
	}
	if auth.ServiceType == "" {
		return nil, fmt.Errorf("vpn key api_config.service_type is empty")
	}
	return auth, nil
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
