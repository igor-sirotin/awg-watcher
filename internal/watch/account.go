package watch

import (
	"encoding/json"
	"sort"
	"strings"
)

func ParseAccountInfo(b []byte) (*AccountInfo, error) {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	info := &AccountInfo{Raw: raw}
	info.AvailableCountries = parseCountries(raw["available_countries"])
	info.IssuedConfigs = parseIssuedConfigs(raw["issued_configs"])
	info.ActiveDeviceCount = intNumber(raw["active_device_count"])
	info.MaxDeviceCount = intNumber(raw["max_device_count"])
	info.SubscriptionEndDate = asString(raw["subscription_end_date"])
	info.SubscriptionDescription = asString(raw["subscription_description"])
	info.SupportedProtocols = stringSlice(raw["supported_protocols"])
	if m, ok := raw["support_info"].(map[string]any); ok {
		info.SupportInfo = m
	}
	return info, nil
}

func (a *AccountInfo) Summary() *AccountSummary {
	if a == nil {
		return nil
	}
	return &AccountSummary{
		AvailableCountries:      a.AvailableCountries,
		IssuedCountryConfigs:    issuedConfigSummaries(a),
		ActiveDeviceCount:       a.ActiveDeviceCount,
		MaxDeviceCount:          a.MaxDeviceCount,
		SubscriptionEndDate:     a.SubscriptionEndDate,
		SubscriptionDescription: a.SubscriptionDescription,
	}
}

func issuedConfigSummaries(info *AccountInfo) []IssuedConfigSummary {
	configs := CountryConfigs(info)
	out := make([]IssuedConfigSummary, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, IssuedConfigSummary{
			Code:              cfg.ServerCountryCode,
			Name:              cfg.ServerCountryName,
			WorkerLastUpdated: cfg.WorkerLastUpdated,
			LastDownloaded:    cfg.LastDownloaded,
			InstallationUUID:  cfg.InstallationUUID,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func CountryConfigs(info *AccountInfo) map[string]IssuedConfig {
	out := map[string]IssuedConfig{}
	for _, c := range info.IssuedConfigs {
		if c.SourceType != "" && c.SourceType != "country_config" {
			continue
		}
		code := strings.ToUpper(c.ServerCountryCode)
		if code != "" {
			c.ServerCountryCode = code
			out[code] = c
		}
	}
	return out
}

func parseCountries(v any) []Country {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]Country, 0, len(arr))
	for _, item := range arr {
		switch x := item.(type) {
		case string:
			out = append(out, Country{Code: strings.ToUpper(x)})
		case map[string]any:
			code := firstString(x, "code", "country_code", "server_country_code")
			name := firstString(x, "name", "country_name", "server_country_name")
			if code != "" {
				out = append(out, Country{Code: strings.ToUpper(code), Name: name})
			}
		}
	}
	return out
}

func parseIssuedConfigs(v any) []IssuedConfig {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]IssuedConfig, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, IssuedConfig{
			ServerCountryCode: strings.ToUpper(asString(m["server_country_code"])),
			ServerCountryName: asString(m["server_country_name"]),
			InstallationUUID:  asString(m["installation_uuid"]),
			WorkerLastUpdated: asString(m["worker_last_updated"]),
			LastDownloaded:    asString(m["last_downloaded"]),
			SourceType:        asString(m["source_type"]),
			OSVersion:         asString(m["os_version"]),
		})
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := asString(m[key]); s != "" {
			return s
		}
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func intNumber(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
