package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds Control HTTP API server settings.
type ServerConfig struct {
	ListenAddr         string   `yaml:"listen_addr"`
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
}

// DatabaseConfig holds SQLite persistence settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ClusterConfig holds distributed worker cluster coordination settings.
type ClusterConfig struct {
	ClusterSecret       string `yaml:"cluster_secret"`
	HeartbeatTimeoutSec int    `yaml:"heartbeat_timeout_sec"`
	ReconcileOnConnect  bool   `yaml:"reconcile_on_connect"`
}

// AuthConfig holds token and crypto settings.
type AuthConfig struct {
	Ed25519PrivateKey string `yaml:"ed25519_private_key"` // 64-byte hex or base64
	JWTExpireHours    int    `yaml:"jwt_expire_hours"`
	TicketTTLSec      int    `yaml:"ticket_ttl_sec"`
}

// TemplatesConfig holds lab declarative templates directory.
type TemplatesConfig struct {
	Dir string `yaml:"dir"`
}

// Config represents top-level Control Hub configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Cluster   ClusterConfig   `yaml:"cluster"`
	Auth      AuthConfig      `yaml:"auth"`
	Templates TemplatesConfig `yaml:"templates"`
}

// DefaultConfig returns default control configurations.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddr: "127.0.0.1:8080",
			CORSAllowedOrigins: []string{
				"http://127.0.0.1",
				"http://localhost",
				"http://localhost:5173",
			},
		},
		Database: DatabaseConfig{
			Path: "/var/lib/attackbox/attackbox.db",
		},
		Cluster: ClusterConfig{
			ClusterSecret:       "default_dev_cluster_secret_32_chars_long",
			HeartbeatTimeoutSec: 10,
			ReconcileOnConnect:  true,
		},
		Auth: AuthConfig{
			Ed25519PrivateKey: "", // Auto-generated if empty
			JWTExpireHours:    24,
			TicketTTLSec:      60,
		},
		Templates: TemplatesConfig{
			Dir: "configs/templates",
		},
	}
}

// LoadConfig parses a control YAML file or falls back to defaults.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("config file not found at %s (using defaults)", path)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}

	return cfg, nil
}
