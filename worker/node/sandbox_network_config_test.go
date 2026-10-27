package node

import (
	"strings"
	"testing"
)

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
