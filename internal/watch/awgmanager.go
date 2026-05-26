package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const awgManagerHTTPErrorBodyLimit = 512

type AWGManagerClient struct {
	Config     AWGManagerConfig
	HTTPClient *http.Client
}

func (c AWGManagerClient) Health(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, "/health", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c AWGManagerClient) ListTunnels(ctx context.Context) ([]AWGManagerTunnel, error) {
	var tunnels []AWGManagerTunnel
	if err := c.requestJSON(ctx, http.MethodGet, "/tunnels/list", nil, &tunnels); err != nil {
		return nil, err
	}
	return tunnels, nil
}

func (c AWGManagerClient) ExportTunnel(ctx context.Context, id string) ([]byte, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("missing tunnel id")
	}
	return c.requestRaw(ctx, http.MethodGet, "/tunnels/export?id="+url.QueryEscape(id), nil)
}

func (c AWGManagerClient) ReplaceTunnel(ctx context.Context, id, content, name string) (map[string]any, []string, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil, fmt.Errorf("missing tunnel id")
	}
	if strings.TrimSpace(content) == "" {
		return nil, nil, fmt.Errorf("missing config content")
	}
	req := map[string]string{"content": content, "name": name}
	var out map[string]any
	if err := c.requestJSON(ctx, http.MethodPost, "/tunnels/replace?id="+url.QueryEscape(id), req, &out); err != nil {
		return nil, nil, err
	}
	var warnings []string
	if raw, ok := out["warnings"].([]any); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok {
				warnings = append(warnings, s)
			}
		}
	}
	return out, warnings, nil
}

func (c AWGManagerClient) requestJSON(ctx context.Context, method, path string, body any, out any) error {
	raw, err := c.requestRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err == nil {
		if rawSuccess, ok := envelope["success"]; ok {
			var success bool
			_ = json.Unmarshal(rawSuccess, &success)
			if !success {
				return fmt.Errorf("%s", awgManagerEnvelopeError(envelope))
			}
			if out == nil {
				return nil
			}
			if data := envelope["data"]; len(data) > 0 && string(data) != "null" {
				return json.Unmarshal(data, out)
			}
			return json.Unmarshal(raw, out)
		}
		if rawError, ok := envelope["error"]; ok {
			var isError bool
			if err := json.Unmarshal(rawError, &isError); err == nil && isError {
				return fmt.Errorf("%s", awgManagerEnvelopeError(envelope))
			}
		}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func awgManagerEnvelopeError(envelope map[string]json.RawMessage) string {
	for _, key := range []string{"message", "error"} {
		var msg string
		if raw := envelope[key]; len(raw) > 0 && json.Unmarshal(raw, &msg) == nil && msg != "" {
			if code := awgManagerEnvelopeCode(envelope); code != "" {
				return msg + " (" + code + ")"
			}
			return msg
		}
	}
	if code := awgManagerEnvelopeCode(envelope); code != "" {
		return "awg-manager request failed (" + code + ")"
	}
	return "awg-manager request failed"
}

func awgManagerEnvelopeCode(envelope map[string]json.RawMessage) string {
	var code string
	_ = json.Unmarshal(envelope["code"], &code)
	return code
}

func (c AWGManagerClient) requestRaw(ctx context.Context, method, path string, body any) ([]byte, error) {
	cfg := c.Config
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultAWGManagerBaseURL
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		jar, _ := cookiejar.New(nil)
		httpClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
	}
	if cfg.Login != "" || cfg.Password != "" {
		if err := awgManagerLogin(ctx, httpClient, cfg); err != nil {
			return nil, err
		}
	}
	endpoint, err := awgManagerEndpoint(cfg.BaseURL, path)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
		return nil, fmt.Errorf("awg-manager returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(limitBytes(respBody, awgManagerHTTPErrorBodyLimit))))
	}
	return respBody, nil
}

func awgManagerLogin(ctx context.Context, httpClient *http.Client, cfg AWGManagerConfig) error {
	if strings.TrimSpace(cfg.Login) == "" || strings.TrimSpace(cfg.Password) == "" {
		return fmt.Errorf("awg-manager login/password are required")
	}
	endpoint, err := awgManagerEndpoint(cfg.BaseURL, "/auth/login")
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"login": cfg.Login, "password": cfg.Password})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("awg-manager login returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(limitBytes(respBody, awgManagerHTTPErrorBodyLimit))))
	}
	return nil
}

func awgManagerEndpoint(baseURL, path string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("awg-manager base URL must include scheme and host")
	}
	basePath := strings.TrimRight(u.Path, "/")
	if basePath == "" {
		basePath = "/api"
	}
	if !strings.HasSuffix(basePath, "/api") {
		basePath += "/api"
	}
	pathPart, queryPart, _ := strings.Cut(strings.TrimLeft(path, "/"), "?")
	u.Path = basePath + "/" + pathPart
	u.RawQuery = queryPart
	return u.String(), nil
}

func PreviewAWGConfig(content string) (AWGConfigPreview, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return AWGConfigPreview{}, fmt.Errorf("missing config content")
	}
	sections := parseWireGuardConfig(content)
	preview := AWGConfigPreview{
		Interface: map[string]string{},
		Peers:     []map[string]string{},
	}
	for _, section := range sections {
		switch strings.ToLower(section.name) {
		case "interface":
			for key, value := range section.values {
				preview.Interface[key] = redactConfigField(key, value)
			}
		case "peer":
			peer := map[string]string{}
			for key, value := range section.values {
				peer[key] = redactConfigField(key, value)
			}
			preview.Peers = append(preview.Peers, peer)
		}
	}
	if len(preview.Interface) == 0 {
		preview.Warnings = append(preview.Warnings, "missing [Interface] section")
	}
	if len(preview.Peers) == 0 {
		preview.Warnings = append(preview.Warnings, "missing [Peer] section")
	}
	if len(preview.Warnings) > 0 {
		return preview, errors.New(strings.Join(preview.Warnings, "; "))
	}
	return preview, nil
}

type wireGuardSection struct {
	name   string
	values map[string]string
}

func parseWireGuardConfig(content string) []wireGuardSection {
	var sections []wireGuardSection
	current := -1
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, wireGuardSection{name: strings.TrimSpace(line[1 : len(line)-1]), values: map[string]string{}})
			current = len(sections) - 1
			continue
		}
		if current < 0 {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		sections[current].values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return sections
}

func redactConfigField(key, value string) string {
	lower := strings.ToLower(key)
	switch {
	case strings.Contains(lower, "privatekey"), strings.Contains(lower, "presharedkey"):
		return redactString(value)
	default:
		return value
	}
}

func (a *App) awgClient() AWGManagerClient {
	a.mu.Lock()
	cfg := a.cfg.AWGManager
	a.mu.Unlock()
	return AWGManagerClient{Config: cfg}
}

func (a *App) ReplaceAWGTunnel(ctx context.Context, id, content, name string) (*AWGReplaceResult, error) {
	if _, err := PreviewAWGConfig(content); err != nil {
		return nil, err
	}
	client := a.awgClient()
	backup, err := client.ExportTunnel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("backup current tunnel: %w", err)
	}
	backupPath, err := a.saveAWGBackup(id, backup)
	if err != nil {
		return nil, fmt.Errorf("save tunnel backup: %w", err)
	}
	tunnel, warnings, err := client.ReplaceTunnel(ctx, id, content, name)
	if err != nil {
		return nil, err
	}
	return &AWGReplaceResult{BackupPath: backupPath, Tunnel: tunnel, Warnings: warnings}, nil
}

func (a *App) saveAWGBackup(id string, content []byte) (string, error) {
	dir := a.paths.AWGBackupDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(a.paths.StatePath), "backups")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	filename := time.Now().UTC().Format("20060102T150405Z") + "-" + sanitizeBackupName(id) + ".conf"
	path := filepath.Join(dir, filename)
	return path, os.WriteFile(path, content, 0600)
}

var backupNameRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizeBackupName(name string) string {
	name = backupNameRE.ReplaceAllString(strings.TrimSpace(name), "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "tunnel"
	}
	return name
}
