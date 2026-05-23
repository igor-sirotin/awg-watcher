package watch

import "time"

const (
	DefaultListenAddr        = "127.0.0.1:8097"
	DefaultConfigPath        = "/opt/etc/amnezia-config-watch/config.json"
	DefaultStatePath         = "/opt/var/lib/amnezia-config-watch/state.json"
	DefaultGatewayEndpoint   = "http://gw.amnezia.org:80/"
	DefaultTelegramEndpoint  = "https://api.telegram.org"
	defaultPollIntervalHours = 6
)

type Paths struct {
	ConfigPath string
	StatePath  string
}

func DefaultPaths() Paths {
	return Paths{ConfigPath: DefaultConfigPath, StatePath: DefaultStatePath}
}

type Config struct {
	ListenAddr        string         `json:"listen_addr"`
	VPNKey            string         `json:"vpn_key"`
	Countries         []string       `json:"countries"`
	PollIntervalHours int            `json:"poll_interval_hours"`
	Telegram          TelegramConfig `json:"telegram"`
	Amnezia           AmneziaConfig  `json:"amnezia"`
	Web               WebConfig      `json:"web"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	Endpoint string `json:"endpoint,omitempty"`
}

type AmneziaConfig struct {
	GatewayEndpoint  string `json:"gateway_endpoint"`
	GatewayPublicKey string `json:"gateway_public_key,omitempty"`
}

type WebConfig struct {
	PasswordHash string `json:"password_hash,omitempty"`
}

type State struct {
	LastCheck    time.Time               `json:"last_check,omitempty"`
	Status       string                  `json:"status"`
	LastError    string                  `json:"last_error,omitempty"`
	ErrorCount   int                     `json:"error_count,omitempty"`
	Countries    map[string]CountryState `json:"countries"`
	LastAccount  *AccountSummary         `json:"last_account,omitempty"`
	LastNotified map[string]string       `json:"last_notified,omitempty"`
}

type CountryState struct {
	Code              string    `json:"code"`
	Name              string    `json:"name,omitempty"`
	WorkerLastUpdated string    `json:"worker_last_updated,omitempty"`
	LastDownloaded    string    `json:"last_downloaded,omitempty"`
	InstallationUUID  string    `json:"installation_uuid,omitempty"`
	Status            string    `json:"status"`
	LastChange        time.Time `json:"last_change,omitempty"`
	Message           string    `json:"message,omitempty"`
}

type AccountInfo struct {
	AvailableCountries      []Country      `json:"available_countries"`
	IssuedConfigs           []IssuedConfig `json:"issued_configs"`
	ActiveDeviceCount       int            `json:"active_device_count,omitempty"`
	MaxDeviceCount          int            `json:"max_device_count,omitempty"`
	SubscriptionEndDate     string         `json:"subscription_end_date,omitempty"`
	SubscriptionDescription string         `json:"subscription_description,omitempty"`
	SupportedProtocols      []string       `json:"supported_protocols,omitempty"`
	SupportInfo             map[string]any `json:"support_info,omitempty"`
	Raw                     map[string]any `json:"-"`
}

type Country struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`
}

type IssuedConfig struct {
	ServerCountryCode string `json:"server_country_code"`
	ServerCountryName string `json:"server_country_name,omitempty"`
	InstallationUUID  string `json:"installation_uuid,omitempty"`
	WorkerLastUpdated string `json:"worker_last_updated,omitempty"`
	LastDownloaded    string `json:"last_downloaded,omitempty"`
	SourceType        string `json:"source_type,omitempty"`
	OSVersion         string `json:"os_version,omitempty"`
}

type AccountSummary struct {
	AvailableCountries      []Country `json:"available_countries"`
	ActiveDeviceCount       int       `json:"active_device_count,omitempty"`
	MaxDeviceCount          int       `json:"max_device_count,omitempty"`
	SubscriptionEndDate     string    `json:"subscription_end_date,omitempty"`
	SubscriptionDescription string    `json:"subscription_description,omitempty"`
}

type CheckResult struct {
	Status   string          `json:"status"`
	Messages []string        `json:"messages"`
	State    *State          `json:"state,omitempty"`
	Account  *AccountSummary `json:"account,omitempty"`
}
