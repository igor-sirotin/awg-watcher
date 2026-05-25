package watch

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

const gatewayHTTPErrorBodyLimit = 512

type AccountClient struct {
	Config      *Config
	FixturePath string
	HTTPClient  *http.Client
}

func (c AccountClient) FetchAccountInfo(ctx context.Context) (*AccountInfo, error) {
	if c.FixturePath != "" {
		b, err := os.ReadFile(c.FixturePath)
		if err != nil {
			return nil, err
		}
		return ParseAccountInfo(b)
	}
	if c.Config == nil {
		return nil, fmt.Errorf("missing config")
	}
	decoded, err := DecodeVPNKey(c.Config.VPNKey)
	if err != nil {
		return nil, err
	}
	auth, err := ExtractPremiumAuth(decoded)
	if err != nil {
		return nil, err
	}
	publicKeys, err := gatewayPublicKeys(c.Config)
	if err != nil {
		return nil, err
	}
	endpoint, err := joinEndpoint(c.Config.Amnezia.GatewayEndpoint, "v1/account_info")
	if err != nil {
		return nil, err
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	var lastErr error
	for i, publicKey := range publicKeys {
		info, err := c.fetchAccountInfoWithKey(ctx, httpClient, endpoint, auth, publicKey)
		if err == nil {
			return info, nil
		}
		lastErr = err
		if !shouldTryNextGatewayKey(err) || i == len(publicKeys)-1 {
			break
		}
	}
	return nil, lastErr
}

func (c AccountClient) fetchAccountInfoWithKey(ctx context.Context, httpClient *http.Client, endpoint string, auth *PremiumAuth, publicKey *rsa.PublicKey) (*AccountInfo, error) {
	body, key, iv, err := buildGatewayRequest(auth, publicKey)
	if err != nil {
		return nil, err
	}
	respBody, err := postGatewayRequest(ctx, httpClient, endpoint, body)
	if err != nil {
		return nil, err
	}
	plain, err := aesCBCDecrypt(respBody, key, iv[:aes.BlockSize])
	if err != nil {
		return nil, fmt.Errorf("decrypt gateway response: %w", err)
	}
	return ParseAccountInfo(plain)
}

func postGatewayRequest(ctx context.Context, httpClient *http.Client, endpoint string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, gatewayHTTPError{StatusCode: resp.StatusCode, Body: string(limitBytes(respBody, gatewayHTTPErrorBodyLimit))}
	}
	return respBody, nil
}

type gatewayHTTPError struct {
	StatusCode int
	Body       string
}

func (e gatewayHTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("gateway returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("gateway returned HTTP %d: %s", e.StatusCode, body)
}

func shouldTryNextGatewayKey(err error) bool {
	var httpErr gatewayHTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.StatusCode >= 500 && httpErr.StatusCode <= 599
}

func limitBytes(in []byte, limit int) []byte {
	if len(in) <= limit {
		return in
	}
	return append(in[:limit], []byte("...")...)
}

func buildGatewayRequest(auth *PremiumAuth, pub *rsa.PublicKey) ([]byte, []byte, []byte, error) {
	key := make([]byte, 32)
	iv := make([]byte, 32)
	salt := make([]byte, 8)
	if _, err := rand.Read(key); err != nil {
		return nil, nil, nil, err
	}
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, nil, err
	}
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, err
	}
	keyPayload, err := json.Marshal(map[string]string{
		"aes_key":  base64.StdEncoding.EncodeToString(key),
		"aes_iv":   base64.StdEncoding.EncodeToString(iv),
		"aes_salt": base64.StdEncoding.EncodeToString(salt),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	encryptedKey, err := rsa.EncryptPKCS1v15(rand.Reader, pub, keyPayload)
	if err != nil {
		return nil, nil, nil, err
	}
	apiPayload, err := json.Marshal(map[string]any{
		"os_version":        runtime.GOOS,
		"app_version":       "awg-watcher",
		"app_language":      "en",
		"installation_uuid": "awg-watcher",
		"user_country_code": auth.UserCountryCode,
		"service_type":      auth.ServiceType,
		"service_protocol":  auth.ServiceProtocol,
		"auth_data":         auth.AuthData,
		"cli_version":       "awg-watcher",
	})
	if err != nil {
		return nil, nil, nil, err
	}
	encryptedAPI, err := aesCBCEncrypt(apiPayload, key, iv[:aes.BlockSize])
	if err != nil {
		return nil, nil, nil, err
	}
	requestBody, err := json.Marshal(map[string]string{
		"key_payload": base64.StdEncoding.EncodeToString(encryptedKey),
		"api_payload": base64.StdEncoding.EncodeToString(encryptedAPI),
	})
	return requestBody, key, iv, err
}

func gatewayPublicKeys(cfg *Config) ([]*rsa.PublicKey, error) {
	key := ""
	if cfg != nil {
		key = strings.TrimSpace(cfg.Amnezia.GatewayPublicKey)
		if key == "" && cfg.Amnezia.GatewayPublicKeyFilePath != "" {
			b, err := os.ReadFile(cfg.Amnezia.GatewayPublicKeyFilePath)
			if err != nil {
				return nil, fmt.Errorf("read gateway public key file %s: %w", cfg.Amnezia.GatewayPublicKeyFilePath, err)
			}
			key = strings.TrimSpace(string(b))
		}
	}
	if key == "" {
		return nil, fmt.Errorf("gateway public key is not configured")
	}
	return gatewayPublicKeysFromPEM(key)
}

func gatewayPublicKeysFromPEM(key string) ([]*rsa.PublicKey, error) {
	var keys []*rsa.PublicKey
	rest := []byte(key)
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if block.Type != "PUBLIC KEY" && block.Type != "RSA PUBLIC KEY" {
			continue
		}
		parsed, err := parsePublicKeyBlock(block)
		if err != nil {
			return nil, err
		}
		keys = append(keys, parsed)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("gateway public key file does not contain a PEM public key")
	}
	return keys, nil
}

func GatewayPublicKeyFileStatus(path string) map[string]any {
	status := map[string]any{
		"path":       path,
		"configured": false,
		"count":      0,
	}
	b, err := os.ReadFile(path)
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	keys, err := gatewayPublicKeysFromPEM(strings.TrimSpace(string(b)))
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	status["configured"] = len(keys) > 0
	status["count"] = len(keys)
	delete(status, "error")
	return status
}

func parsePublicKeyBlock(block *pem.Block) (*rsa.PublicKey, error) {
	if block.Type == "RSA PUBLIC KEY" {
		return x509.ParsePKCS1PublicKey(block.Bytes)
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("gateway public key is not RSA")
	}
	return publicKey, nil
}

func aesCBCEncrypt(plain, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain = pkcs7Pad(plain, block.BlockSize())
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out, nil
}

func aesCBCDecrypt(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("invalid CBC ciphertext length")
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ciphertext)
	return pkcs7Unpad(out, block.BlockSize())
}

func pkcs7Pad(in []byte, blockSize int) []byte {
	n := blockSize - (len(in) % blockSize)
	out := make([]byte, len(in)+n)
	copy(out, in)
	for i := len(in); i < len(out); i++ {
		out[i] = byte(n)
	}
	return out
}

func pkcs7Unpad(in []byte, blockSize int) ([]byte, error) {
	if len(in) == 0 || len(in)%blockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS padding length")
	}
	n := int(in[len(in)-1])
	if n == 0 || n > blockSize || n > len(in) {
		return nil, fmt.Errorf("invalid PKCS padding")
	}
	for _, b := range in[len(in)-n:] {
		if int(b) != n {
			return nil, fmt.Errorf("invalid PKCS padding")
		}
	}
	return in[:len(in)-n], nil
}

func joinEndpoint(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return u.String(), nil
}
