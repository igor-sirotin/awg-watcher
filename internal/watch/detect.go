package watch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func DetectChanges(cfg *Config, st *State, info *AccountInfo) CheckResult {
	now := time.Now().UTC()
	if st.Countries == nil {
		st.Countries = map[string]CountryState{}
	}
	if st.LastNotified == nil {
		st.LastNotified = map[string]string{}
	}

	issued := CountryConfigs(info)
	watched := normalizeCountries(cfg.Countries)
	messages := []string{}
	status := "ok"

	if len(watched) == 0 {
		st.Status = "unknown"
		st.LastCheck = now
		st.LastError = ""
		st.ErrorCount = 0
		st.LastAccount = info.Summary()
		return CheckResult{Status: "unknown", Messages: []string{"no watched countries configured"}, State: st, Account: info.Summary()}
	}

	for _, code := range watched {
		prev, existed := st.Countries[code]
		current, ok := issued[code]
		if !ok {
			status = "changed"
			prev.Code = code
			prev.Status = "missing"
			prev.Message = "watched country missing from account info"
			prev.LastChange = now
			st.Countries[code] = prev
			messages = append(messages, fmt.Sprintf("%s missing from issued configs", code))
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
			messages = append(messages, fmt.Sprintf("%s baseline created", code))
		} else if prev.WorkerLastUpdated != current.WorkerLastUpdated || prev.LastDownloaded != current.LastDownloaded {
			status = "changed"
			next.Status = "changed"
			next.Message = "country config metadata changed"
			next.LastChange = now
			messages = append(messages, fmt.Sprintf("%s metadata changed", code))
		} else {
			next.LastChange = prev.LastChange
		}
		st.Countries[code] = next
	}

	st.Status = status
	st.LastCheck = now
	st.LastError = ""
	st.ErrorCount = 0
	st.LastAccount = info.Summary()
	return CheckResult{Status: status, Messages: messages, State: st, Account: info.Summary()}
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

func NotificationText(result CheckResult) string {
	if len(result.Messages) == 0 {
		return ""
	}
	return "Amnezia Config Watcher:\n" + strings.Join(result.Messages, "\n")
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
	client := AccountClient{Config: &cfg, FixturePath: a.fixturePath}
	info, err := client.FetchAccountInfo(ctx)
	if err != nil {
		MarkCheckError(st, err)
		_ = SaveState(a.paths.StatePath, st)
		if notify && st.ErrorCount >= 3 {
			_ = SendTelegram(ctx, cfg.Telegram, fmt.Sprintf("Amnezia Config Watcher: repeated API failures (%d): %s", st.ErrorCount, err))
		}
		return &CheckResult{Status: st.Status, Messages: []string{err.Error()}, State: st}, err
	}
	result := DetectChanges(&cfg, st, info)
	if err := SaveState(a.paths.StatePath, st); err != nil {
		return nil, err
	}
	if notify {
		msg := NotificationText(result)
		if msg != "" {
			_ = SendTelegram(ctx, cfg.Telegram, msg)
		}
	}
	return &result, nil
}
