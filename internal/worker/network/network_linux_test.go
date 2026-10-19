//go:build linux

package network_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"attackbox/internal/worker/config"
	"attackbox/internal/worker/network"
)

func TestHostNetworkAndVethLifecycle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test: requires root privileges for network manipulation")
	}

	cfg := config.NetworkConfig{
		BridgeName:         "br0",
		BridgeIP:           "10.200.0.1/16",
		TargetBridge:       "docker0",
		WANInterface:       "",
		EgressAllowedPorts: []int{53, 80, 443},
	}

	// 1. Test Host Network Initialization
	t.Log("Initializing host bridge and firewall baseline...")
	if err := network.EnsureHostNetwork(cfg); err != nil {
		t.Fatalf("EnsureHostNetwork failed: %v", err)
	}

	// Verify br0 exists
	if out, err := exec.Command("ip", "link", "show", "br0").CombinedOutput(); err != nil {
		t.Fatalf("br0 not found: %s (%v)", strings.TrimSpace(string(out)), err)
	}

	// Verify nftables table exists
	if out, err := exec.Command("nft", "list", "table", "inet", "attackbox").CombinedOutput(); err != nil {
		t.Fatalf("nft table attackbox not found: %s (%v)", strings.TrimSpace(string(out)), err)
	}

	// 2. Test Sandbox NetNS & Veth Provisioning
	testNetns := "ab-test-ns99"
	_ = exec.Command("ip", "netns", "del", testNetns).Run()
	if out, err := exec.Command("ip", "netns", "add", testNetns).CombinedOutput(); err != nil {
		t.Fatalf("failed to create test netns %s: %s (%v)", testNetns, strings.TrimSpace(string(out)), err)
	}
	defer func() {
		_ = exec.Command("ip", "netns", "del", testNetns).Run()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slotIndex := 99
	t.Logf("Setting up sandbox veth pair for slot %d in netns %s...", slotIndex, testNetns)
	sbxNet, err := network.SetupSandboxNetwork(ctx, "br0", slotIndex, testNetns)
	if err != nil {
		t.Fatalf("SetupSandboxNetwork failed: %v", err)
	}
	t.Logf("Network provisioned: IP=%s, HostDev=%s, GuestDev=%s", sbxNet.IP, sbxNet.VethHost, sbxNet.VethGuest)

	// 3. Test Connectivity: Ping gateway from inside the sandbox network namespace
	t.Log("Testing ping from container netns to gateway 10.200.0.1...")
	pingCmd := exec.CommandContext(ctx, "ip", "netns", "exec", testNetns, "ping", "-c", "2", "-W", "1", "10.200.0.1")
	if out, err := pingCmd.CombinedOutput(); err != nil {
		t.Fatalf("ping gateway failed: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	t.Log("Ping to gateway succeeded!")

	// 4. Test Teardown
	t.Log("Testing sandbox network teardown...")
	if err := network.TeardownSandboxNetwork(ctx, slotIndex); err != nil {
		t.Fatalf("TeardownSandboxNetwork failed: %v", err)
	}

	// Verify host veth is gone
	if err := exec.Command("ip", "link", "show", sbxNet.VethHost).Run(); err == nil {
		t.Fatalf("expected host veth %s to be deleted, but still exists", sbxNet.VethHost)
	}
	t.Log("Veth pair removed cleanly!")
}
