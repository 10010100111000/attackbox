package control

import (
	"attackbox/protocol"
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type ControlConfig struct {
	Server struct {
		ListenAddr         string   `yaml:"listen_addr"`
		CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Cluster struct {
		ClusterSecret       string `yaml:"cluster_secret"`
		HeartbeatTimeoutSec int    `yaml:"heartbeat_timeout_sec"`
		RPCTimeoutSec       int    `yaml:"rpc_timeout_sec"`
	} `yaml:"cluster"`
	Auth struct {
		Ed25519PrivateKey    string `yaml:"ed25519_private_key"`
		JWTExpireHours       int    `yaml:"jwt_expire_hours"`
		TicketTTLSec         int    `yaml:"ticket_ttl_sec"`
		InitialAdminUsername string `yaml:"initial_admin_username"`
		InitialAdminPassword string `yaml:"initial_admin_password"`
		InitialAdminEmail    string `yaml:"initial_admin_email"`
	} `yaml:"auth"`
	Sandbox struct {
		Resources         protocol.BoxResourceQuota `yaml:"resources"`
		Storage           protocol.BoxStoragePolicy `yaml:"storage"`
		TTLSeconds        int                       `yaml:"ttl_seconds"`
		IdleSeconds       int                       `yaml:"idle_seconds"`
		ReaperIntervalSec int                       `yaml:"reaper_interval_sec"`
	} `yaml:"sandbox"`
}

// SanitizeAndApplyDefaults enforces fail-closed production baselines on any ControlConfig.
func (cfg *ControlConfig) SanitizeAndApplyDefaults() {
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = "127.0.0.1:18080"
	}
	if len(cfg.Server.CORSAllowedOrigins) == 0 {
		cfg.Server.CORSAllowedOrigins = []string{"*"}
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "/var/lib/attackbox/attackbox.db"
	}
	if cfg.Cluster.ClusterSecret == "" {
		cfg.Cluster.ClusterSecret = "attackbox_cluster_secret_change_me_in_prod!"
	}
	if cfg.Cluster.HeartbeatTimeoutSec <= 0 {
		cfg.Cluster.HeartbeatTimeoutSec = 15
	}
	if cfg.Cluster.RPCTimeoutSec <= 0 {
		cfg.Cluster.RPCTimeoutSec = 30
	}
	if cfg.Auth.JWTExpireHours <= 0 {
		cfg.Auth.JWTExpireHours = 24
	}
	if cfg.Auth.TicketTTLSec <= 0 {
		cfg.Auth.TicketTTLSec = 60
	}
	if cfg.Auth.InitialAdminUsername == "" {
		cfg.Auth.InitialAdminUsername = "admin"
	}
	if cfg.Auth.InitialAdminPassword == "" {
		cfg.Auth.InitialAdminPassword = "AdminSecurePassword123!"
	}
	if cfg.Auth.InitialAdminEmail == "" {
		cfg.Auth.InitialAdminEmail = "admin@attackbox.local"
	}

	// Fail-closed baseline resource enforcement
	if cfg.Sandbox.Resources.CPUWeight == 0 {
		cfg.Sandbox.Resources.CPUWeight = protocol.DefaultCPUWeight
	}
	if cfg.Sandbox.Resources.MemoryMaxMB <= 0 {
		cfg.Sandbox.Resources.MemoryMaxMB = protocol.DefaultMemLimitMB
	}
	if cfg.Sandbox.Resources.PIDsMax <= 0 {
		cfg.Sandbox.Resources.PIDsMax = protocol.DefaultPIDsLimit
	}
	if cfg.Sandbox.Resources.IOWriteBytesSec <= 0 {
		cfg.Sandbox.Resources.IOWriteBytesSec = protocol.DefaultIOWriteBytesSec
	}
	if cfg.Sandbox.Resources.IOWriteIOPS <= 0 {
		cfg.Sandbox.Resources.IOWriteIOPS = protocol.DefaultIOWriteIOPS
	}
	if cfg.Sandbox.Storage.TmpfsTmpSize == "" {
		cfg.Sandbox.Storage.TmpfsTmpSize = protocol.DefaultTmpfsTmpSize
	}
	if cfg.Sandbox.Storage.TmpfsRunSize == "" {
		cfg.Sandbox.Storage.TmpfsRunSize = protocol.DefaultTmpfsRunSize
	}
	if cfg.Sandbox.TTLSeconds <= 0 {
		cfg.Sandbox.TTLSeconds = 7200 // 2 hours hard lifetime ceiling
	}
	if cfg.Sandbox.IdleSeconds <= 0 {
		cfg.Sandbox.IdleSeconds = 1800 // 30 minutes inactivity timeout
	}
	if cfg.Sandbox.ReaperIntervalSec <= 0 {
		cfg.Sandbox.ReaperIntervalSec = 30 // 30 seconds scanning period
	}
}

// LoadControlConfig reads YAML configuration or falls back to production defaults.
func LoadControlConfig(path string) (*ControlConfig, error) {
	cfg := &ControlConfig{}
	cfg.SanitizeAndApplyDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read control config %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}
	cfg.SanitizeAndApplyDefaults()
	return cfg, nil
}
