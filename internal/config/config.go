package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Modems    ModemsConfig    `yaml:"modems"`
	Proxy     ProxyConfig     `yaml:"proxy"`
	Rotation  RotationConfig  `yaml:"rotation"`
	Storage   StorageConfig   `yaml:"storage"`
	WireGuard WireGuardConfig `yaml:"wireguard"`
}

type ServerConfig struct {
	Host     string `yaml:"host"`
	APIPort  int    `yaml:"api_port"`
	LogLevel string `yaml:"log_level"`
}

type AuthConfig struct {
	Users map[string]string `yaml:"users"`
}

type ModemsConfig struct {
	ScanInterval Duration       `yaml:"scan_interval"`
	AutoConnect  bool           `yaml:"auto_connect"`
	DefaultAPN   string         `yaml:"default_apn"`
	Overrides    []ModemOverride `yaml:"overrides"`
}

type ModemOverride struct {
	IMEI  string `yaml:"imei"`
	APN   string `yaml:"apn"`
	Label string `yaml:"label"`
}

type ProxyConfig struct {
	HTTPPortStart  int    `yaml:"http_port_start"`
	SOCKS5PortStart int   `yaml:"socks5_port_start"`
	AuthRequired   bool   `yaml:"auth_required"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
}

type RotationConfig struct {
	Cooldown     Duration `yaml:"cooldown"`
	Timeout      Duration `yaml:"timeout"`
	AutoRotate   bool     `yaml:"auto_rotate"`
	AutoInterval Duration `yaml:"auto_interval"`
}

type StorageConfig struct {
	DatabasePath  string `yaml:"database_path"`
	RetentionDays int    `yaml:"retention_days"`
}

type WireGuardConfig struct {
	Enabled       bool   `yaml:"enabled"`
	VPSEndpoint   string `yaml:"vps_endpoint"`
	VPSPublicKey  string `yaml:"vps_public_key"`
	LocalPrivKey  string `yaml:"local_private_key"`
	TunnelSubnet  string `yaml:"tunnel_subnet"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "0.0.0.0",
			APIPort:  8080,
			LogLevel: "info",
		},
		Auth: AuthConfig{
			Users: map[string]string{"admin": "changeme"},
		},
		Modems: ModemsConfig{
			ScanInterval: Duration{30 * time.Second},
			AutoConnect:  true,
			DefaultAPN:   "internet",
		},
		Proxy: ProxyConfig{
			HTTPPortStart:   8901,
			SOCKS5PortStart: 1081,
			AuthRequired:    true,
			Username:        "proxy",
			Password:        "changeme",
		},
		Rotation: RotationConfig{
			Cooldown:     Duration{10 * time.Second},
			Timeout:      Duration{30 * time.Second},
			AutoInterval: Duration{5 * time.Minute},
		},
		Storage: StorageConfig{
			DatabasePath:  "./data/modemux.db",
			RetentionDays: 30,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}

func FindAndLoad() (*Config, error) {
	paths := []string{
		"./config.yaml",
		"/etc/modemux/config.yaml",
	}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, home+"/.config/modemux/config.yaml")
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return Load(p)
		}
	}

	return Default(), nil
}

func (c *Config) WarnDefaultCredentials() {
	for user, pass := range c.Auth.Users {
		if pass == "changeme" {
			slog.Warn("default credentials detected, change immediately",
				"user", user, "config_key", "auth.users")
		}
	}
	if c.Proxy.Password == "changeme" {
		slog.Warn("default proxy password detected, change immediately",
			"config_key", "proxy.password")
	}
}
