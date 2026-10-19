package node

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// NetworkConfig holds network virtualization settings for sandboxes.
type NetworkConfig struct {
	BridgeName string `yaml:"bridge_name"`
	GatewayIP  string `yaml:"gateway_ip"`
	SubnetCIDR string `yaml:"subnet_cidr"`
	Interface  string `yaml:"egress_interface"`
}

// SandboxNetwork represents the network configuration of a single running sandbox.
type SandboxNetwork struct {
	SlotIndex int    `json:"slot_index"`
	IP        string `json:"ip"`
	VethHost  string `json:"veth_host"`
	VethGuest string `json:"veth_guest"`
	Netns     string `json:"netns"`
}

// EnsureHostNetwork initializes the host bridge, IPv4 forwarding, NAT, and nftables containment baseline.
func EnsureHostNetwork(cfg NetworkConfig) error {
	bridge := cfg.BridgeName
	if bridge == "" {
		bridge = "br-attackbox"
	}
	gatewayIP := cfg.GatewayIP
	if gatewayIP == "" {
		gatewayIP = "10.200.0.1"
	}
	bridgeCIDR := gatewayIP + "/16"
	targetBridge := "docker0"

	// 1. Enable IPv4 kernel forwarding
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644)
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	// 2. Idempotently create bridge device
	if err := exec.Command("ip", "link", "show", bridge).Run(); err != nil {
		_ = exec.Command("ip", "link", "add", bridge, "type", "bridge").Run()
		_ = exec.Command("ip", "addr", "add", bridgeCIDR, "dev", bridge).Run()
		_ = exec.Command("ip", "link", "set", bridge, "up").Run()
	} else {
		_ = exec.Command("ip", "link", "set", bridge, "up").Run()
	}

	// 3. Detect default WAN interface if not specified
	wanIF := cfg.Interface
	if wanIF == "" {
		wanIF = detectDefaultWANInterface()
	}

	// 4. Build allowed WAN egress port list
	allowedPorts := []int{53, 80, 443}
	var portStrs []string
	for _, p := range allowedPorts {
		portStrs = append(portStrs, strconv.Itoa(p))
	}
	portList := strings.Join(portStrs, ", ")

	// 5. Construct nftables fail-closed isolation rules
	nftRules := fmt.Sprintf(`
table inet attackbox {
    chain forward {
        type filter hook forward priority 0; policy drop;
        ct state established,related accept

        # Inter-sandbox isolation: reject any traffic between sandboxes
        iifname "%s" oifname "%s" drop

        # Bidirectional containment: Sandbox <-> Target Bridge
        iifname "%s" oifname "%s" accept
        iifname "%s" oifname "%s" accept

        # Controlled Internet Egress: Allow sandbox to reach WAN only on whitelisted ports
        iifname "%s" oifname "%s" tcp dport { %s } accept
        iifname "%s" oifname "%s" udp dport { 53 } accept
    }

    chain postrouting {
        type nat hook postrouting priority 100; policy accept;
        # Outbound masquerade to Internet
        iifname "%s" oifname "%s" masquerade
    }
}
`, bridge, bridge, bridge, targetBridge, targetBridge, bridge, bridge, wanIF, portList, bridge, wanIF, bridge, wanIF)

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewReader([]byte(nftRules))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apply nftables baseline: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// SetupSandboxNetwork provisions an isolated veth pair, attaches to the bridge with port isolation,
// and moves the guest interface into the sandbox's network namespace.
func SetupSandboxNetwork(ctx context.Context, bridge string, slotIndex int, targetNetns string) (*SandboxNetwork, error) {
	if bridge == "" {
		bridge = "br-attackbox"
	}
	if slotIndex <= 0 {
		slotIndex = 1
	}

	// Safety guard: strictly ensure targetNetns is NOT the host's root network namespace (/proc/1/ns/net)
	hostNetns, _ := os.Readlink("/proc/1/ns/net")
	targetReal, _ := os.Readlink(targetNetns)
	if hostNetns != "" && targetReal != "" && hostNetns == targetReal {
		return nil, fmt.Errorf("safety violation: targetNetns %s points to host root netns (%s), refusing to modify host routing", targetNetns, hostNetns)
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

	// 5. Rename guest interface to eth0 inside the sandbox netns
	cmdRename := exec.CommandContext(ctx, "ip", "netns", "exec", targetNetns, "ip", "link", "set", vethGuest, "name", "eth0")
	if out, err := cmdRename.CombinedOutput(); err != nil {
		cmdRenameFile := exec.CommandContext(ctx, "nsenter", "--net="+targetNetns, "ip", "link", "set", vethGuest, "name", "eth0")
		if out2, err2 := cmdRenameFile.CombinedOutput(); err2 != nil {
			_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
			return nil, fmt.Errorf("rename guest veth to eth0 in %s: %s / %s (%w)", targetNetns, string(out), string(out2), err2)
		}
	}

	// 6. Assign IP address and set eth0 UP inside sandbox
	cmdIP := exec.CommandContext(ctx, "nsenter", "--net="+targetNetns, "ip", "addr", "add", boxIP, "dev", "eth0")
	if out, err := cmdIP.CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("set eth0 ip %s in %s: %s (%w)", boxIP, targetNetns, strings.TrimSpace(string(out)), err)
	}

	cmdUp := exec.CommandContext(ctx, "nsenter", "--net="+targetNetns, "ip", "link", "set", "eth0", "up")
	if out, err := cmdUp.CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("set eth0 up in %s: %s (%w)", targetNetns, strings.TrimSpace(string(out)), err)
	}

	// 7. Enable Loopback device inside sandbox
	_ = exec.CommandContext(ctx, "nsenter", "--net="+targetNetns, "ip", "link", "set", "lo", "up").Run()

	// 8. Set default gateway to bridge IP (10.200.0.1)
	cmdRoute := exec.CommandContext(ctx, "nsenter", "--net="+targetNetns, "ip", "route", "add", "default", "via", "10.200.0.1")
	if out, err := cmdRoute.CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("set default route in %s: %s (%w)", targetNetns, strings.TrimSpace(string(out)), err)
	}

	assignedIP := strings.Split(boxIP, "/")[0]
	return &SandboxNetwork{
		SlotIndex: slotIndex,
		IP:        assignedIP,
		VethHost:  vethHost,
		VethGuest: "eth0",
		Netns:     targetNetns,
	}, nil
}

// TeardownSandboxNetwork deletes the host veth peer, automatically cleaning up the kernel veth pair.
func TeardownSandboxNetwork(ctx context.Context, slotIndex int) error {
	vethHost := fmt.Sprintf("vh%d", slotIndex)
	cmd := exec.CommandContext(ctx, "ip", "link", "del", vethHost)
	_ = cmd.Run()
	return nil
}

func detectDefaultWANInterface() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return "eth0"
}
