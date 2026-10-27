package node

import (
	rt "attackbox/node/runtime"
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type NodeInfoConfig struct {
	NodeID               string `yaml:"node_id"`
	ClusterSecret        string `yaml:"cluster_secret"`
	ControlURL           string `yaml:"control_url"`
	HeartbeatIntervalSec int    `yaml:"heartbeat_interval_sec"`
}

type NodeConfig struct {
	ComposeLabs ComposeConfig    `yaml:"compose_labs"`
	Node        NodeInfoConfig   `yaml:"node"`
	Gateway     GatewayConfig    `yaml:"gateway"`
	Runtime     rt.RuntimeConfig `yaml:"runtime"`
	Network     NetworkConfig    `yaml:"network"`
}

// LoadNodeConfig requires an explicit file; omitted host fields use local defaults.
func LoadNodeConfig(path string) (*NodeConfig, error) {
	cfg := &NodeConfig{
		Node: NodeInfoConfig{
			NodeID:               "node-primary",
			ControlURL:           "ws://127.0.0.1:3000/cluster/connect",
			HeartbeatIntervalSec: 3,
		},
		Gateway: GatewayConfig{
			ListenAddr: "0.0.0.0:8443",
			TLSEnabled: false,
		},
		Runtime: rt.RuntimeConfig{
			BaseRootfs:   "/var/lib/attackbox/base_rootfs",
			InstancesDir: "/var/lib/attackbox/instances",
			CgroupRoot:   "/sys/fs/cgroup/attackbox",
			DiskGuard: rt.DiskGuardConfig{
				Enabled:          true,
				CheckIntervalSec: 3,
				MinFreeSpaceMB:   5120,
				WarnFreeSpaceMB:  10240,
			},
		},
		Network: NetworkConfig{
			BridgeName: "br-attackbox",
			GatewayIP:  "10.200.0.1",
			SubnetCIDR: "10.200.0.0/16",
			Interface:  "",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read node config %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}
	if err := validateSandboxNetwork(cfg.Network); err != nil {
		return nil, err
	}
	return cfg, nil
}

// GatewayConfig represents external interactive gateway listener parameters.
type GatewayConfig struct {
	ListenAddr       string   `yaml:"listen_addr"`
	TLSEnabled       bool     `yaml:"tls_enabled"`
	TLSCert          string   `yaml:"tls_cert"`
	TLSKey           string   `yaml:"tls_key"`
	Ed25519PublicKey string   `yaml:"ed25519_public_key"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	PublicDataURL    string   `yaml:"public_data_url"`
}
