package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// NodeConfig represents worker node identity and cluster connection parameters.
type NodeConfig struct {
	NodeID               string `yaml:"node_id"`
	ClusterSecret        string `yaml:"cluster_secret"`
	ControlURL           string `yaml:"control_url"`
	HeartbeatIntervalSec int    `yaml:"heartbeat_interval_sec"`
}

// DataServerConfig represents external data channel listener configuration.
type DataServerConfig struct {
	ListenAddr       string `yaml:"listen_addr"`
	TLSEnabled       bool   `yaml:"tls_enabled"`
	TLSCert          string `yaml:"tls_cert"`
	TLSKey           string `yaml:"tls_key"`
	Ed25519PublicKey string `yaml:"ed25519_public_key"`
}

// ResourceLimitsConfig represents configurable sandbox limits.
type ResourceLimitsConfig struct {
	PIDsMax          int64  `yaml:"pids_max"`            // Max processes allowed inside sandbox (0 = unlimited)
	MemoryMaxMB      int64  `yaml:"memory_max_mb"`       // Physical RAM limit in MB (0 = unlimited)
	MemorySwapMaxMB  int64  `yaml:"memory_swap_max_mb"`  // Swap limit in MB (0 = disable swap)
	CPUWeight        uint64 `yaml:"cpu_weight"`          // Cgroups v2 cpu.weight (1 - 10000)
	IOWriteBytesSec  int64  `yaml:"io_write_bytes_sec"`  // Max disk write bytes/sec (e.g. 20971520 = 20MB/s; 0 = unlimited)
	IOWriteIOPS      int64  `yaml:"io_write_iops"`       // Max disk write IOPS (0 = unlimited)
	RootDeviceNumber string `yaml:"root_device_number"`  // Major:Minor of root btrfs device (e.g. "254:1", empty to auto-detect)
}

// TmpfsConfig represents configurable memory-backed filesystem sizes.
type TmpfsConfig struct {
	TmpSize string `yaml:"tmp_size"` // tmpfs size for /tmp (e.g. "512M")
	RunSize string `yaml:"run_size"` // tmpfs size for /run (e.g. "256M")
}

// DiskGuardConfig represents host disk capacity watermark monitoring and auto-fuse settings.
type DiskGuardConfig struct {
	Enabled          bool  `yaml:"enabled"`
	CheckIntervalSec int   `yaml:"check_interval_sec"`
	MinFreeSpaceMB   int64 `yaml:"min_free_space_mb"`  // Circuit breaker threshold (stop accepting & alert)
	WarnFreeSpaceMB  int64 `yaml:"warn_free_space_mb"` // Warning threshold reported to Control
}

// RuntimeConfig represents sandbox container runtime settings.
type RuntimeConfig struct {
	BaseRootfs   string               `yaml:"base_rootfs"`   // Golden base read-only btrfs subvolume
	InstancesDir string               `yaml:"instances_dir"` // Dynamic ephemeral instances subvolumes root
	CgroupRoot   string               `yaml:"cgroup_root"`   // Cgroups v2 base hierarchy
	Limits       ResourceLimitsConfig `yaml:"limits"`
	Tmpfs        TmpfsConfig          `yaml:"tmpfs"`
	DiskGuard    DiskGuardConfig      `yaml:"disk_guard"`
}

// NetworkConfig represents bridge and port routing settings.
type NetworkConfig struct {
	BridgeName         string   `yaml:"bridge_name"`
	BridgeIP           string   `yaml:"bridge_ip"`
	TargetBridge       string   `yaml:"target_bridge"`
	WANInterface       string   `yaml:"wan_interface"`
	EgressAllowedPorts []int    `yaml:"egress_allowed_ports"`
}

// Config is the top-level Worker configuration struct.
type Config struct {
	Node       NodeConfig       `yaml:"node"`
	DataServer DataServerConfig `yaml:"data_server"`
	Runtime    RuntimeConfig    `yaml:"runtime"`
	Network    NetworkConfig    `yaml:"network"`
}

// DefaultConfig returns a fully-populated Config with production-grade defaults.
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			NodeID:               "worker-node-1",
			ClusterSecret:        "",
			ControlURL:           "ws://127.0.0.1:8080/api/agent/connect",
			HeartbeatIntervalSec: 3,
		},
		DataServer: DataServerConfig{
			ListenAddr: ":8443",
			TLSEnabled: false,
		},
		Runtime: RuntimeConfig{
			BaseRootfs:   "/var/lib/attackbox/base_rootfs",
			InstancesDir: "/var/lib/attackbox/instances",
			CgroupRoot:   "/sys/fs/cgroup/attackbox",
			Limits: ResourceLimitsConfig{
				PIDsMax:          512,
				MemoryMaxMB:      2048,
				MemorySwapMaxMB:  0,
				CPUWeight:        100,
				IOWriteBytesSec:  20 * 1024 * 1024, // 20 MB/s
				IOWriteIOPS:      500,
				RootDeviceNumber: "", // Auto-detect
			},
			Tmpfs: TmpfsConfig{
				TmpSize: "512M",
				RunSize: "256M",
			},
			DiskGuard: DiskGuardConfig{
				Enabled:          true,
				CheckIntervalSec: 3,
				MinFreeSpaceMB:   5120,  // 5 GB
				WarnFreeSpaceMB:  10240, // 10 GB
			},
		},
		Network: NetworkConfig{
			BridgeName:         "br0",
			BridgeIP:           "10.200.0.1/16",
			TargetBridge:       "docker0",
			WANInterface:       "eth0",
			EgressAllowedPorts: []int{53, 80, 443},
		},
	}
}

// LoadConfig parses a YAML configuration file from disk.
// If the file does not exist, defaults are used and a helpful error is returned.
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
