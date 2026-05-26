package watch

import (
	"path/filepath"
	"time"
)

const (
	DefaultListenAddr           = "127.0.0.1:8097"
	DefaultConfigPath           = "/opt/etc/awg-watcher/config.json"
	DefaultStatePath            = "/opt/var/lib/awg-watcher/state.json"
	DefaultGatewayPublicKeyPath = "/opt/etc/awg-watcher/gateway_public_key.pem"
	DefaultGatewayEndpoint      = "http://gw.amnezia.org:80/"
	DefaultAWGManagerBaseURL    = "http://127.0.0.1:2222/api"
	DefaultTelegramEndpoint     = "https://api.telegram.org"
	defaultPollIntervalHours    = 6
)

type Paths struct {
	ConfigPath           string
	StatePath            string
	GatewayPublicKeyPath string
	AWGBackupDir         string
}

func DefaultPaths() Paths {
	return Paths{
		ConfigPath:           DefaultConfigPath,
		StatePath:            DefaultStatePath,
		GatewayPublicKeyPath: DefaultGatewayPublicKeyPath,
		AWGBackupDir:         "/opt/var/lib/awg-watcher/backups",
	}
}

func (p *Paths) ApplyWorkdir(workdir string) {
	if workdir == "" {
		return
	}
	p.ConfigPath = filepath.Join(workdir, "config.json")
	p.StatePath = filepath.Join(workdir, "state.json")
	p.AWGBackupDir = filepath.Join(workdir, "backups")
	if p.GatewayPublicKeyPath == "" || p.GatewayPublicKeyPath == DefaultGatewayPublicKeyPath {
		p.GatewayPublicKeyPath = filepath.Join(workdir, "gateway_public_key.pem")
	}
}

type Config struct {
	ListenAddr        string           `json:"listen_addr"`
	VPNKey            string           `json:"vpn_key"`
	Countries         []string         `json:"countries"`
	Keys              []KeyConfig      `json:"keys"`
	PollIntervalHours int              `json:"poll_interval_hours"`
	Telegram          TelegramConfig   `json:"telegram"`
	Amnezia           AmneziaConfig    `json:"amnezia"`
	AWGManager        AWGManagerConfig `json:"awg_manager"`
	Web               WebConfig        `json:"web"`
}

type KeyConfig struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	VPNKey    string   `json:"vpn_key"`
	Countries []string `json:"countries"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	Endpoint string `json:"endpoint,omitempty"`
}

type AmneziaConfig struct {
	GatewayEndpoint          string `json:"gateway_endpoint"`
	GatewayPublicKey         string `json:"gateway_public_key,omitempty"`
	GatewayPublicKeyFilePath string `json:"gateway_public_key_filepath,omitempty"`
}

type AWGManagerConfig struct {
	BaseURL  string `json:"base_url"`
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
}

type WebConfig struct {
	PasswordHash string `json:"password_hash,omitempty"`
}

type AWGManagerTunnel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Status      string `json:"status,omitempty"`
	Enabled     bool   `json:"enabled,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Address     string `json:"address,omitempty"`
	Interface   string `json:"interfaceName,omitempty"`
	NDMSName    string `json:"ndmsName,omitempty"`
	Backend     string `json:"backend,omitempty"`
	BackendType string `json:"backendType,omitempty"`
	MTU         int    `json:"mtu,omitempty"`
}

type AWGConfigPreview struct {
	Interface map[string]string   `json:"interface"`
	Peers     []map[string]string `json:"peers"`
	Warnings  []string            `json:"warnings,omitempty"`
}

type AWGReplaceResult struct {
	BackupPath string         `json:"backup_path"`
	Tunnel     map[string]any `json:"tunnel,omitempty"`
	Warnings   []string       `json:"warnings,omitempty"`
}

type State struct {
	LastCheck    time.Time               `json:"last_check,omitempty"`
	Status       string                  `json:"status"`
	LastError    string                  `json:"last_error,omitempty"`
	ErrorCount   int                     `json:"error_count,omitempty"`
	Countries    map[string]CountryState `json:"countries"`
	Keys         map[string]KeyState     `json:"keys,omitempty"`
	LastAccount  *AccountSummary         `json:"last_account,omitempty"`
	LastNotified map[string]string       `json:"last_notified,omitempty"`
}

type KeyState struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	LastCheck   time.Time               `json:"last_check,omitempty"`
	Status      string                  `json:"status"`
	LastError   string                  `json:"last_error,omitempty"`
	ErrorCount  int                     `json:"error_count,omitempty"`
	Countries   map[string]CountryState `json:"countries"`
	LastAccount *AccountSummary         `json:"last_account,omitempty"`
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
	AvailableCountries      []Country             `json:"available_countries"`
	IssuedCountryConfigs    []IssuedConfigSummary `json:"issued_country_configs,omitempty"`
	ActiveDeviceCount       int                   `json:"active_device_count,omitempty"`
	MaxDeviceCount          int                   `json:"max_device_count,omitempty"`
	SubscriptionEndDate     string                `json:"subscription_end_date,omitempty"`
	SubscriptionDescription string                `json:"subscription_description,omitempty"`
}

type CheckResult struct {
	Status   string           `json:"status"`
	Messages []string         `json:"messages"`
	State    *State           `json:"state,omitempty"`
	Account  *AccountSummary  `json:"account,omitempty"`
	Keys     []KeyCheckResult `json:"keys,omitempty"`
}

type KeyCheckResult struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Status   string          `json:"status"`
	Messages []string        `json:"messages"`
	Account  *AccountSummary `json:"account,omitempty"`
}

type IssuedConfigSummary struct {
	Code              string `json:"code"`
	Name              string `json:"name,omitempty"`
	WorkerLastUpdated string `json:"worker_last_updated,omitempty"`
	LastDownloaded    string `json:"last_downloaded,omitempty"`
	InstallationUUID  string `json:"installation_uuid,omitempty"`
}
