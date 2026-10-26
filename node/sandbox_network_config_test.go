package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeOnlyConfigRejectsRetiredNetworkSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.yaml")
	os.WriteFile(path, []byte("network:\n  docker_targets:\n    enabled: true\n"), 0600)
	if _, err := LoadNodeConfig(path); err == nil {
		t.Fatal("retired network configuration accepted")
	}
}
func TestSandboxFirewallHasNoRetiredCrossBridgeExceptions(t *testing.T) {
	cfg := NetworkConfig{BridgeName: "br-attackbox", SubnetCIDR: "10.200.0.0/16", GatewayIP: "10.200.0.1", Interface: "eth0"}
	if err := validateSandboxNetwork(cfg); err != nil {
		t.Fatal(err)
	}
	rules := renderFirewallRules("attackbox", cfg.BridgeName, cfg.Interface, "53,80,443")
	if strings.Contains(rules, "br-ab-targets") || strings.Contains(rules, "10.201.") {
		t.Fatal("retired target forwarding retained")
	}
	for _, want := range []string{`iifname "br-attackbox" oifname "br-attackbox" drop`, `iifname "br-attackbox" drop`, `masquerade`} {
		if !strings.Contains(rules, want) {
			t.Fatal("sandbox containment removed", want)
		}
	}
}
func TestRetiredTargetLedgerMustBeCleanBeforeUpgrade(t *testing.T) {
	root := t.TempDir()
	instances := filepath.Join(root, "instances")
	state := filepath.Join(root, "docker-target-state")
	os.Mkdir(state, 0700)
	record := filepath.Join(state, "old.json")
	for _, body := range []string{`{"cleaned":false}`, `invalid`} {
		os.WriteFile(record, []byte(body), 0600)
		if checkRetiredTargets(instances) == nil {
			t.Fatal("unconfirmed cleanup ignored")
		}
	}
	os.WriteFile(record, []byte(`{"cleaned":true}`), 0600)
	if err := checkRetiredTargets(instances); err != nil {
		t.Fatal(err)
	}
}
