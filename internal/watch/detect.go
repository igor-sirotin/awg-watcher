package watch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func DetectChanges(cfg *Config, st *State, info *AccountInfo) CheckResult {
	key := KeyConfig{ID: "default", Name: "Default key", Countries: cfg.Countries}
	keyState := detectKeyChanges(key, KeyState{ID: key.ID, Name: key.Name, Countries: st.Countries}, info)
	st.Countries = keyState.Countries
	st.Status = keyState.Status
	st.LastCheck = keyState.LastCheck
	st.LastError = keyState.LastError
	st.ErrorCount = keyState.ErrorCount
	st.LastAccount = keyState.LastAccount
	return CheckResult{Status: keyState.Status, Messages: keyStateMessages(keyState), State: st, Account: info.Summary()}
}

func detectKeyChanges(key KeyConfig, prevState KeyState, info *AccountInfo) KeyState {
	now := time.Now().UTC()
	if prevState.Countries == nil {
		prevState.Countries = map[string]CountryState{}
	}

	issued := CountryConfigs(info)
	watched := normalizeCountries(key.Countries)
	status := "ok"
	nextState := KeyState{
		ID:          key.ID,
		Name:        key.Name,
		LastCheck:   now,
		Status:      status,
		Countries:   prevState.Countries,
		LastAccount: info.Summary(),
	}

	if len(watched) == 0 {
		nextState.Status = "unknown"
		nextState.Countries = map[string]CountryState{}
		return nextState
	}

	for _, code := range watched {
		prev, existed := prevState.Countries[code]
		current, ok := issued[code]
		if !ok {
			status = "changed"
			prev.Code = code
			prev.Status = "missing"
			prev.Message = "watched country missing from account info"
			prev.LastChange = now
			nextState.Countries[code] = prev
			continue
		}

		next := CountryState{
			Code:              code,
			Name:              current.ServerCountryName,
			WorkerLastUpdated: current.WorkerLastUpdated,
			LastDownloaded:    current.LastDownloaded,
			InstallationUUID:  current.InstallationUUID,
			Status:            "ok",
		}
		if !existed || prev.WorkerLastUpdated == "" && prev.LastDownloaded == "" {
			next.Status = "baseline"
			next.Message = "baseline created"
			next.LastChange = now
		} else if prev.WorkerLastUpdated != current.WorkerLastUpdated || prev.LastDownloaded != current.LastDownloaded {
			status = "changed"
			next.Status = "changed"
			next.Message = "country config metadata changed"
			next.LastChange = now
		} else {
			next.LastChange = prev.LastChange
		}
		nextState.Countries[code] = next
	}

	nextState.Status = status
	nextState.LastError = ""
	nextState.ErrorCount = 0
	return nextState
}

func MarkCheckError(st *State, err error) {
	st.LastCheck = time.Now().UTC()
	st.Status = "api_error"
	st.LastError = err.Error()
	st.ErrorCount++
	if st.Countries == nil {
		st.Countries = map[string]CountryState{}
	}
}

func markKeyCheckError(key KeyConfig, prevState KeyState, err error) KeyState {
	if prevState.Countries == nil {
		prevState.Countries = map[string]CountryState{}
	}
	prevState.ID = key.ID
	prevState.Name = key.Name
	prevState.LastCheck = time.Now().UTC()
	prevState.Status = "api_error"
	prevState.LastError = err.Error()
	prevState.ErrorCount++
	return prevState
}

func NotificationText(result CheckResult) string {
	if len(result.Messages) == 0 {
		return ""
	}
	return "Amnezia Config Watcher:\n" + strings.Join(result.Messages, "\n")
}

func keyStateMessages(st KeyState) []string {
	var messages []string
	switch st.Status {
	case "unknown":
		messages = append(messages, fmt.Sprintf("%s: no watched countries configured", st.Name))
	case "api_error":
		messages = append(messages, fmt.Sprintf("%s: %s", st.Name, st.LastError))
	}
	for _, country := range sortedCountryStates(st.Countries) {
		switch country.Status {
		case "baseline":
			messages = append(messages, fmt.Sprintf("%s %s baseline created", st.Name, country.Code))
		case "changed":
			messages = append(messages, fmt.Sprintf("%s %s metadata changed", st.Name, country.Code))
		case "missing":
			messages = append(messages, fmt.Sprintf("%s %s missing from issued configs", st.Name, country.Code))
		}
	}
	return messages
}

func sortedCountryStates(countries map[string]CountryState) []CountryState {
	out := make([]CountryState, 0, len(countries))
	for _, country := range countries {
		out = append(out, country)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func normalizeCountries(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func (a *App) Check(ctx context.Context, notify bool) (*CheckResult, error) {
	a.mu.Lock()
	cfg := *a.cfg
	a.mu.Unlock()

	st, err := LoadState(a.paths.StatePath)
	if err != nil {
		return nil, err
	}

	if st.Keys == nil {
		st.Keys = map[string]KeyState{}
	}
	keys := cfg.Keys
	if len(keys) == 0 && strings.TrimSpace(cfg.VPNKey) != "" {
		keys = []KeyConfig{{ID: "default", Name: "Default key", VPNKey: cfg.VPNKey, Countries: cfg.Countries}}
	}
	if len(keys) == 0 {
		st.LastCheck = time.Now().UTC()
		st.Status = "unknown"
		st.LastError = ""
		if err := SaveState(a.paths.StatePath, st); err != nil {
			return nil, err
		}
		return &CheckResult{Status: "unknown", Messages: []string{"no AmneziaVPN keys configured"}, State: st}, nil
	}

	result := &CheckResult{Status: "ok", State: st}
	var firstErr error
	var allUnknown = true
	var anyChanged, anyError bool
	now := time.Now().UTC()
	for _, key := range keys {
		if strings.TrimSpace(key.ID) == "" {
			key.ID = key.Name
		}
		keyCfg := cfg
		keyCfg.VPNKey = key.VPNKey
		keyCfg.Countries = key.Countries
		client := AccountClient{Config: &keyCfg, FixturePath: a.fixturePath}
		info, err := client.FetchAccountInfo(ctx)
		var keyResult KeyCheckResult
		if err != nil {
			keyState := markKeyCheckError(key, st.Keys[key.ID], err)
			st.Keys[key.ID] = keyState
			keyResult = KeyCheckResult{ID: key.ID, Name: key.Name, Status: keyState.Status, Messages: keyStateMessages(keyState), Account: keyState.LastAccount}
			anyError = true
			if firstErr == nil {
				firstErr = err
			}
			if notify && keyState.ErrorCount >= 3 {
				_ = SendTelegram(ctx, cfg.Telegram, fmt.Sprintf("Amnezia Config Watcher: %s repeated API failures (%d): %s", key.Name, keyState.ErrorCount, err))
			}
		} else {
			keyState := detectKeyChanges(key, st.Keys[key.ID], info)
			st.Keys[key.ID] = keyState
			keyResult = KeyCheckResult{ID: key.ID, Name: key.Name, Status: keyState.Status, Messages: keyStateMessages(keyState), Account: info.Summary()}
			if keyState.Status != "unknown" {
				allUnknown = false
			}
			if keyState.Status == "changed" {
				anyChanged = true
			}
		}
		result.Keys = append(result.Keys, keyResult)
		result.Messages = append(result.Messages, keyResult.Messages...)
	}
	st.LastCheck = now
	st.LastError = ""
	st.ErrorCount = 0
	if anyError {
		st.Status = "api_error"
		st.ErrorCount = 1
		if firstErr != nil {
			st.LastError = firstErr.Error()
		}
	} else if anyChanged {
		st.Status = "changed"
	} else if allUnknown {
		st.Status = "unknown"
	} else {
		st.Status = "ok"
	}
	result.Status = st.Status
	if err := SaveState(a.paths.StatePath, st); err != nil {
		return nil, err
	}
	if notify {
		msg := NotificationText(*result)
		if msg != "" {
			_ = SendTelegram(ctx, cfg.Telegram, msg)
		}
	}
	return result, nil
}
