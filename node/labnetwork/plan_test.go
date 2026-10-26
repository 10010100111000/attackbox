package labnetwork

import (
	"net/netip"
	"strings"
	"testing"
)

func spec(id string) NetworkSpec {
	return NetworkSpec{TargetID: id, OwnerID: "user-a", Key: "front", PrefixBits: 28}
}

func TestPlanReservationsAndOccupiedRanges(t *testing.T) {
	pool := netip.MustParsePrefix("10.201.0.0/16")
	n, err := Plan(pool, spec("tgt_one"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n.Subnet.String() != "10.201.0.0/28" || n.Gateway.String() != "10.201.0.1" || n.SandboxIP.String() != "10.201.0.14" {
		t.Fatalf("bad reservation: %+v", n)
	}
	second, err := Plan(pool, spec("tgt_two"), []netip.Prefix{n.Subnet, netip.MustParsePrefix("10.201.0.16/28")})
	if err != nil {
		t.Fatal(err)
	}
	if second.Subnet.String() != "10.201.0.32/28" {
		t.Fatal(second.Subnet)
	}
	s := spec("big")
	s.PrefixBits = 27
	big, err := Plan(pool, s, []netip.Prefix{n.Subnet})
	if err != nil {
		t.Fatal(err)
	}
	if big.Subnet.String() != "10.201.0.32/27" || big.SandboxIP.String() != "10.201.0.62" {
		t.Fatal(big)
	}
	if _, err := Plan(pool, spec("full"), []netip.Prefix{pool}); err == nil {
		t.Fatal("overlapping pool accepted")
	}
	for _, bad := range []string{"10.201.0.1/16", "::/64", "10.0.0.0/8", "10.201.0.0/29"} {
		if _, err := Plan(netip.MustParsePrefix(bad), spec("bad"), nil); err == nil {
			t.Fatal(bad)
		}
	}
}

func TestNamesAreBoundedAndIncludeNetworkAndNamespaceIdentity(t *testing.T) {
	n, err := Plan(netip.MustParsePrefix("10.201.0.0/16"), spec(strings.Repeat("x", 64)), nil)
	if err != nil {
		t.Fatal(err)
	}
	s := Sandbox{ID: "sandbox", OwnerID: "user-a", PID: 42, Namespace: NamespaceIdentity{Device: 1, Inode: 2}}
	a := makeAttachment("node-a", n, s, 255)
	for _, name := range []string{n.Bridge, a.HostName, a.PeerName, a.GuestName} {
		if len(name) > 15 {
			t.Fatal(name)
		}
	}
	s.Namespace.Inode++
	if a.ID == makeAttachment("node-a", n, s, 255).ID {
		t.Fatal("namespace identity omitted")
	}
	other := n.Spec
	other.Key = "back"
	n2, _ := Plan(n.Subnet, other, nil)
	if n.Bridge == n2.Bridge {
		t.Fatal("network key omitted")
	}
}

func TestProtectionHasEarlyDropsWithoutCrossBridgeAllowOrNAT(t *testing.T) {
	rules, err := ProtectionRules("node-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`iifname "bt_*" drop`, `priority -10`} {
		if !strings.Contains(rules, want) {
			t.Fatal(want)
		}
	}
	if strings.Contains(rules, "masquerade") || strings.Contains(rules, "flush ruleset") || strings.Contains(rules, "DOCKER-USER") || strings.Contains(rules, "chain forward") {
		t.Fatal(rules)
	}
	if _, err := ProtectionRules("bad\";flush ruleset"); err == nil {
		t.Fatal("invalid node accepted")
	}
}
