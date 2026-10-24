package node

import (
	"strings"
	"testing"
)

func TestDedicatedFirewallTableAndForwardScope(t *testing.T) {
	for _, value := range []string{"attackbox_acceptance", "test1"} {
		table, err := firewallTableName(value)
		if err != nil || table != value {
			t.Fatalf("valid table rejected: %v", err)
		}
	}
	for _, value := range []string{"bad; flush ruleset", "bad\nname", "../attackbox", "" + strings.Repeat("x", 33)} {
		if _, err := firewallTableName(value); err == nil {
			t.Fatal("unsafe table accepted")
		}
	}
	table, err := firewallTableName("")
	if err != nil || table != "attackbox" {
		t.Fatal("default changed")
	}
	rules := renderFirewallRules("attackbox_acceptance", "br-abtest", "ens3", "53, 80, 443")
	for _, part := range []string{"table inet attackbox_acceptance", "hook forward priority 0; policy accept", `iifname "br-abtest" drop`, `oifname "br-abtest" drop`, `iifname "br-abtest" oifname "ens3" tcp dport { 53, 80, 443 } accept`} {
		if !strings.Contains(rules, part) {
			t.Fatalf("missing containment rule %q", part)
		}
	}
	if strings.Contains(rules, "policy drop") || strings.Contains(rules, "flush ruleset") {
		t.Fatal("rules modify unrelated host forwarding")
	}
}
