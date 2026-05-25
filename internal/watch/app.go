package watch

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type App struct {
	paths       *Paths
	cfg         *Config
	fixturePath string
	setupToken  string
	mu          sync.Mutex
}

func NewApp(paths *Paths, cfg *Config, fixturePath, setupToken string) *App {
	applyDefaults(cfg)
	return &App{paths: paths, cfg: cfg, fixturePath: fixturePath, setupToken: setupToken}
}

func (a *App) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	a.routes(mux)

	a.mu.Lock()
	addr := a.cfg.ListenAddr
	interval := time.Duration(a.cfg.PollIntervalHours) * time.Hour
	a.mu.Unlock()

	server := &http.Server{Addr: addr, Handler: a.authMiddleware(mux)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go a.scheduler(ctx, interval)

	log.Printf("serving on http://%s", addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *App) scheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = defaultPollIntervalHours * time.Hour
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			checkCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			if _, err := a.Check(checkCtx, true); err != nil {
				log.Printf("scheduled check failed: %s", err)
			}
			cancel()
			timer.Reset(interval)
		}
	}
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		hash := a.cfg.Web.PasswordHash
		token := a.setupToken
		a.mu.Unlock()

		if hash == "" {
			if strings.HasPrefix(r.URL.Path, "/api/") && token != "" {
				got := r.Header.Get("X-Setup-Token")
				if got == "" {
					got = r.URL.Query().Get("setup_token")
				}
				if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
					writeError(w, http.StatusUnauthorized, "setup token required")
					return
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		_, pass, ok := r.BasicAuth()
		if !ok || bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="awg-watcher"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) routes(mux *http.ServeMux) {
	assets, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(assets)))
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/settings", a.handleSettings)
	mux.HandleFunc("/api/decode", a.handleDecode)
	mux.HandleFunc("/api/check", a.handleCheck)
	mux.HandleFunc("/api/telegram/test", a.handleTelegramTest)
	mux.HandleFunc("/api/diagnostics", a.handleDiagnostics)
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	a.mu.Lock()
	cfg := *a.cfg
	setupMode := a.cfg.Web.PasswordHash == ""
	a.mu.Unlock()
	st, _ := LoadState(a.paths.StatePath)
	writeJSON(w, map[string]any{
		"config":                    RedactValue(cfg),
		"state":                     st,
		"next_check":                nextCheckTime(st, cfg),
		"setup_mode":                setupMode,
		"setup_requirements":        setupRequirements(cfg, a.paths, setupMode),
		"gateway_public_key_status": GatewayPublicKeyFileStatus(cfg.Amnezia.GatewayPublicKeyFilePath),
		"fixture":                   a.fixturePath != "",
	})
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ListenAddr        string         `json:"listen_addr"`
		VPNKey            string         `json:"vpn_key"`
		Countries         []string       `json:"countries"`
		Keys              []KeyConfig    `json:"keys"`
		PollIntervalHours int            `json:"poll_interval_hours"`
		Telegram          TelegramConfig `json:"telegram"`
		Amnezia           AmneziaConfig  `json:"amnezia"`
		WebPassword       string         `json:"web_password"`
		GatewayPublicKeys string         `json:"gateway_public_keys"`
		AutoSelectIssued  bool           `json:"auto_select_issued_countries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.mu.Lock()
	cfg := *a.cfg
	wasSetupComplete := isSetupComplete(cfg, a.paths, cfg.Web.PasswordHash == "")
	if req.ListenAddr != "" {
		cfg.ListenAddr = req.ListenAddr
	}
	cfg.VPNKey = mergeSecret(cfg.VPNKey, req.VPNKey)
	cfg.Countries = normalizeCountries(req.Countries)
	if req.Keys != nil {
		keys, err := mergeKeyConfigs(cfg.Keys, req.Keys)
		if err != nil {
			a.mu.Unlock()
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfg.Keys = keys
		cfg.VPNKey = ""
		cfg.Countries = nil
	}
	if req.PollIntervalHours > 0 {
		cfg.PollIntervalHours = req.PollIntervalHours
	}
	cfg.Telegram.BotToken = mergeSecret(cfg.Telegram.BotToken, req.Telegram.BotToken)
	cfg.Telegram.ChatID = mergeSecret(cfg.Telegram.ChatID, req.Telegram.ChatID)
	cfg.Telegram.Endpoint = mergeSecret(cfg.Telegram.Endpoint, req.Telegram.Endpoint)
	cfg.Amnezia.GatewayEndpoint = mergeSecret(cfg.Amnezia.GatewayEndpoint, req.Amnezia.GatewayEndpoint)
	cfg.Amnezia.GatewayPublicKeyFilePath = mergeSecret(cfg.Amnezia.GatewayPublicKeyFilePath, req.Amnezia.GatewayPublicKeyFilePath)
	cfg.Amnezia.GatewayPublicKey = mergeSecret(cfg.Amnezia.GatewayPublicKey, req.Amnezia.GatewayPublicKey)
	if strings.TrimSpace(req.GatewayPublicKeys) != "" {
		path := cfg.Amnezia.GatewayPublicKeyFilePath
		if path == "" {
			path = a.paths.GatewayPublicKeyPath
			cfg.Amnezia.GatewayPublicKeyFilePath = path
		}
		if err := saveGatewayPublicKeyFile(path, req.GatewayPublicKeys); err != nil {
			a.mu.Unlock()
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.WebPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.WebPassword), bcrypt.DefaultCost)
		if err != nil {
			a.mu.Unlock()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		cfg.Web.PasswordHash = string(hash)
		a.setupToken = ""
	}
	passwordChanged := req.WebPassword != ""
	applyDefaults(&cfg)
	if req.AutoSelectIssued {
		a.mu.Unlock()
		if err := a.autoSelectIssuedCountries(r.Context(), &cfg); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		a.mu.Lock()
	}
	if err := SaveConfig(a.paths.ConfigPath, &cfg); err != nil {
		a.mu.Unlock()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.cfg = &cfg
	isNowSetupComplete := isSetupComplete(cfg, a.paths, cfg.Web.PasswordHash == "")
	a.mu.Unlock()
	if !wasSetupComplete && isNowSetupComplete {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = SendTelegram(ctx, cfg.Telegram, "AWG Watcher setup completed")
		}()
	}
	writeJSON(w, map[string]any{
		"ok":               true,
		"config":           RedactValue(cfg),
		"setup_complete":   isNowSetupComplete,
		"password_changed": passwordChanged,
	})
}

func (a *App) autoSelectIssuedCountries(ctx context.Context, cfg *Config) error {
	for i := range cfg.Keys {
		if len(cfg.Keys[i].Countries) > 0 {
			continue
		}
		keyCfg := *cfg
		keyCfg.VPNKey = cfg.Keys[i].VPNKey
		client := AccountClient{Config: &keyCfg, FixturePath: a.fixturePath}
		info, err := client.FetchAccountInfo(ctx)
		if err != nil {
			return fmt.Errorf("%s: fetch account info: %w", cfg.Keys[i].Name, err)
		}
		countries := issuedCountryCodes(info)
		if len(countries) == 0 {
			continue
		}
		cfg.Keys[i].Countries = countries
	}
	return nil
}

func issuedCountryCodes(info *AccountInfo) []string {
	configs := CountryConfigs(info)
	countries := make([]string, 0, len(configs))
	for code := range configs {
		countries = append(countries, code)
	}
	return normalizeCountries(countries)
}

func (a *App) handleDecode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		VPNKey string `json:"vpn_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	decoded, err := DecodeVPNKey(req.VPNKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	auth, authErr := ExtractPremiumAuth(decoded)
	resp := map[string]any{"decoded": RedactValue(decoded)}
	if authErr == nil {
		resp["premium"] = map[string]string{
			"service_type":      auth.ServiceType,
			"service_protocol":  auth.ServiceProtocol,
			"user_country_code": auth.UserCountryCode,
		}
	}
	writeJSON(w, resp)
}

func (a *App) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	result, err := a.Check(ctx, true)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, result)
}

func (a *App) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	a.mu.Lock()
	cfg := a.cfg.Telegram
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := SendTelegram(ctx, cfg, "AWG Watcher test notification"); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	a.mu.Lock()
	cfg := *a.cfg
	a.mu.Unlock()
	st, _ := LoadState(a.paths.StatePath)
	w.Header().Set("Content-Disposition", `attachment; filename="awg-watcher-diagnostics.json"`)
	writeJSON(w, map[string]any{
		"config":  RedactValue(cfg),
		"state":   st,
		"fixture": a.fixturePath != "",
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSONStatus(w, code, map[string]any{"error": msg})
}

func setupRequirements(cfg Config, paths *Paths, setupMode bool) map[string]any {
	gatewayStatus := GatewayPublicKeyFileStatus(cfg.Amnezia.GatewayPublicKeyFilePath)
	hasGatewayKeys := gatewayStatus["configured"] == true || strings.TrimSpace(cfg.Amnezia.GatewayPublicKey) != ""
	return map[string]any{
		"admin_password":      !setupMode,
		"gateway_public_keys": hasGatewayKeys,
		"amnezia_keys":        len(cfg.Keys) > 0,
		"gateway_key_path":    paths.GatewayPublicKeyPath,
	}
}

func isSetupComplete(cfg Config, paths *Paths, setupMode bool) bool {
	req := setupRequirements(cfg, paths, setupMode)
	return req["admin_password"] == true && req["gateway_public_keys"] == true && req["amnezia_keys"] == true
}

func nextCheckTime(st *State, cfg Config) *time.Time {
	if st == nil || st.LastCheck.IsZero() {
		return nil
	}
	next := st.LastCheck.Add(time.Duration(cfg.PollIntervalHours) * time.Hour)
	return &next
}

func mergeKeyConfigs(existing, incoming []KeyConfig) ([]KeyConfig, error) {
	existingByID := map[string]KeyConfig{}
	for _, key := range existing {
		existingByID[key.ID] = key
	}
	out := make([]KeyConfig, 0, len(incoming))
	seen := map[string]bool{}
	for i := range incoming {
		key := incoming[i]
		key.ID = strings.TrimSpace(key.ID)
		if key.ID == "" {
			id, err := GenerateKeyID()
			if err != nil {
				return nil, err
			}
			key.ID = id
		}
		if seen[key.ID] {
			return nil, fmt.Errorf("duplicate key id %s", key.ID)
		}
		seen[key.ID] = true
		old := existingByID[key.ID]
		key.Name = strings.TrimSpace(key.Name)
		if key.Name == "" {
			key.Name = old.Name
		}
		if key.Name == "" {
			key.Name = fmt.Sprintf("Key %d", len(out)+1)
		}
		key.VPNKey = mergeSecret(old.VPNKey, key.VPNKey)
		key.Countries = normalizeCountries(key.Countries)
		if strings.TrimSpace(key.VPNKey) == "" {
			return nil, fmt.Errorf("%s is missing vpn key", key.Name)
		}
		out = append(out, key)
	}
	return out, nil
}

func saveGatewayPublicKeyFile(path, body string) error {
	body = strings.TrimSpace(body)
	if _, err := gatewayPublicKeysFromPEM(body); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body+"\n"), 0600)
}

func HashPasswordForTest(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(fmt.Sprintf("hash password: %v", err))
	}
	return string(hash)
}
