package network

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SandboxNetwork represents the network configuration of a single running sandbox.
type SandboxNetwork struct {
	SlotIndex int    `json:"slot_index"`
	IP        string `json:"ip"`
	VethHost  string `json:"veth_host"`
	VethGuest string `json:"veth_guest"`
	Netns     string `json:"netns"`
}

// SetupSandboxNetwork provisions an isolated veth pair, attaches to the bridge with port isolation,
// and moves the guest interface into the sandbox's network namespace.
func SetupSandboxNetwork(ctx context.Context, bridge string, slotIndex int, targetNetns string) (*SandboxNetwork, error) {
	if bridge == "" {
		bridge = "br0"
	}
	if slotIndex <= 0 {
		slotIndex = 1
	}

	vethHost := fmt.Sprintf("vh%d", slotIndex)
	vethGuest := fmt.Sprintf("vb%d", slotIndex)
	boxIP := fmt.Sprintf("10.200.0.%d/16", 10+slotIndex)

	// Clean up any stale interfaces from previous crashes
	_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()

	// 1. Create veth pair (host peer and guest peer)
	if out, err := exec.CommandContext(ctx, "ip", "link", "add", vethGuest, "type", "veth", "peer", "name", vethHost).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("create veth pair %s <-> %s: %s (%w)", vethGuest, vethHost, strings.TrimSpace(string(out)), err)
	}

	// 2. Attach host peer to bridge and set UP
	if out, err := exec.CommandContext(ctx, "ip", "link", "set", vethHost, "master", bridge, "up").CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("attach %s to bridge %s: %s (%w)", vethHost, bridge, strings.TrimSpace(string(out)), err)
	}

	// 3. Enable Bridge Port Isolation (prevents inter-sandbox ARP sniffing/spoofing on the same bridge)
	_ = exec.CommandContext(ctx, "ip", "link", "set", vethHost, "type", "bridge_slave", "isolated", "on").Run()

	// 4. Move guest interface into the sandbox network namespace
	if out, err := exec.CommandContext(ctx, "ip", "link", "set", vethGuest, "netns", targetNetns).CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("move %s to netns %s: %s (%w)", vethGuest, targetNetns, strings.TrimSpace(string(out)), err)
	}

	// 5. Configure interfaces inside the target network namespace
	// Bring up loopback
	if err := execNetns(ctx, targetNetns, "ip", "link", "set", "lo", "up"); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("bring up lo in netns %s: %w", targetNetns, err)
	}

	// Assign IP to container interface
	if err := execNetns(ctx, targetNetns, "ip", "addr", "add", boxIP, "dev", vethGuest); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("assign IP %s to %s in netns: %w", boxIP, vethGuest, err)
	}

	// Bring up container interface
	if err := execNetns(ctx, targetNetns, "ip", "link", "set", vethGuest, "up"); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("bring up %s in netns: %w", vethGuest, err)
	}

	// Add default gateway route via bridge IP (10.200.0.1)
	if err := execNetns(ctx, targetNetns, "ip", "route", "add", "default", "via", "10.200.0.1"); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("add default route in netns: %w", err)
	}

	return &SandboxNetwork{
		SlotIndex: slotIndex,
		IP:        boxIP,
		VethHost:  vethHost,
		VethGuest: vethGuest,
		Netns:     targetNetns,
	}, nil
}

// TeardownSandboxNetwork removes the veth pair and cleans up network state.
func TeardownSandboxNetwork(ctx context.Context, slotIndex int) error {
	vethHost := fmt.Sprintf("vh%d", slotIndex)
	// Deleting the host peer automatically removes both ends of the veth pair in the kernel
	_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
	return nil
}

func execNetns(ctx context.Context, netns string, cmd string, args ...string) error {
	fullArgs := append([]string{"netns", "exec", netns, cmd}, args...)
	out, err := exec.CommandContext(ctx, "ip", fullArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %s (%w)", strings.Join(fullArgs, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}
