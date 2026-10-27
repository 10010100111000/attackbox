package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeConfigTemplateContainsOnlyHostConfiguration(t *testing.T) {
	cfg, err := LoadNodeConfig(filepath.Join("..", "..", "deploy", "worker", "worker.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Node.NodeID != "node-primary" || cfg.Network.BridgeName != "br-attackbox" || cfg.Runtime.BaseRootfs == "" {
		t.Fatal("host settings not loaded")
	}
	if cfg.ComposeLabs.Enabled || cfg.ComposeLabs.NetworkVerified {
		t.Fatal("unverified network enabled by default")
	}
}
func TestMissingConfigDoesNotStartWithSharedDefaults(t *testing.T) {
	if _, err := LoadNodeConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing config silently accepted")
	}
}
func TestRemovedRuntimeOverridesAreRejected(t *testing.T) {
	for _, field := range []string{"limits: {memory_max_mb: 1024}", "tmpfs: {tmp_size: 512M}", "sandbox_ttl_sec: 7200", "sandbox_idle_sec: 900"} {
		t.Run(field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "node.yaml")
			if err := os.WriteFile(path, []byte("runtime:\n  "+field+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadNodeConfig(path); err == nil || !strings.Contains(err.Error(), "field") {
				t.Fatal("obsolete runtime setting accepted", err)
			}
		})
	}
}
func TestHostDeviceOverrideRemainsSupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  root_device_number: '8:0'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadNodeConfig(path)
	if err != nil || cfg.Runtime.RootDeviceNumber != "8:0" {
		t.Fatal(cfg, err)
	}
}
