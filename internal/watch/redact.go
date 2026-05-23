package watch

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func RedactValue(v any) any {
	b, _ := json.Marshal(v)
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return v
	}
	return redact(decoded, "")
}

func redact(v any, key string) any {
	lower := strings.ToLower(key)
	if shouldRedactKey(lower) {
		if s, ok := v.(string); ok {
			if s == "" {
				return ""
			}
			return redactString(s)
		}
		return "[redacted]"
	}
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			out[k] = redact(val, k)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = redact(val, key)
		}
		return out
	case string:
		if looksLikeNativeConfig(key, x) {
			return "[redacted]"
		}
		return x
	default:
		return x
	}
}

func shouldRedactKey(key string) bool {
	return key == "vpn_key" ||
		key == "api_key" ||
		key == "bot_token" ||
		key == "gateway_public_key" ||
		strings.Contains(key, "private_key") ||
		strings.Contains(key, "preshared_key")
}

func looksLikeNativeConfig(key, value string) bool {
	lowerKey := strings.ToLower(key)
	if strings.Contains(lowerKey, "config") && strings.Contains(value, "[Interface]") {
		return true
	}
	return strings.Contains(value, "PrivateKey =")
}

func redactString(s string) string {
	if len(s) <= 8 {
		return "[redacted]"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func MustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func mergeSecret(existing, incoming string) string {
	if strings.TrimSpace(incoming) == "" {
		return existing
	}
	return incoming
}

func hasField(m map[string]any, name string) bool {
	_, ok := m[name]
	return ok
}

func isZero(v any) bool {
	return reflect.ValueOf(v).IsZero()
}
