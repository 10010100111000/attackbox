package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// NetworkConfig holds network virtualization settings for sandboxes.
type NetworkConfig struct {
	FirewallTable string `yaml:"firewall_table"`
	BridgeName    string `yaml:"bridge_name"`
	GatewayIP     string `yaml:"gateway_ip"`
	SubnetCIDR    string `yaml:"subnet_cidr"`
	Interface     string `yaml:"egress_interface"`
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
	table, err := firewallTableName(cfg.FirewallTable)
	if err != nil {
		return err
	}
	bridge := cfg.BridgeName
	if bridge == "" {
		bridge = "br-attackbox"
	}
	gatewayIP := cfg.GatewayIP
	if gatewayIP == "" {
		gatewayIP = "10.200.0.1"
	}
	bridgeCIDR := gatewayIP + "/16"

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

	// 5. Construct nftables fail-closed isolation rules:
	// - input chain: Blocks sandboxes from accessing host control/SSH ports (allows only DNS and ping to gateway)
	// - forward chain: Enforces strict inter-sandbox isolation and whitelisted internet egress
	// - postrouting chain: Masquerades authorized egress to the external network
	nftRules := renderFirewallRules(table, bridge, wanIF, portList)

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewReader([]byte(nftRules))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apply nftables baseline: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func firewallTableName(value string) (string, error) {
	if value == "" {
		return "attackbox", nil
	}
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`).MatchString(value) {
		return "", fmt.Errorf("invalid firewall table name")
	}
	return value, nil
}

func renderFirewallRules(table, bridge, wanIF, portList string) string {
	return fmt.Sprintf(`
table inet %s {
    chain input {
        type filter hook input priority 0; policy accept;
        ct state established,related accept

        # Allow container to query host gateway DNS (53) and ping (icmp echo-request)
        iifname "%s" udp dport 53 accept
        iifname "%s" tcp dport 53 accept
        iifname "%s" icmp type echo-request accept

        # Reject all other inbound traffic from sandboxes to host control plane / services
        iifname "%s" drop
    }

    chain forward {
        type filter hook forward priority 0; policy accept;
        ct state established,related accept

        # Inter-sandbox isolation: reject any traffic between sandboxes
        iifname "%s" oifname "%s" drop

        # Controlled Internet Egress: Allow sandbox to reach WAN only on whitelisted ports
        iifname "%s" oifname "%s" tcp dport { %s } accept
        iifname "%s" oifname "%s" udp dport { 53 } accept

        # Contain this bridge without changing unrelated host forwarding.
        iifname "%s" drop
        oifname "%s" drop
    }

    chain postrouting {
        type nat hook postrouting priority 100; policy accept;
        # Outbound masquerade to Internet
        iifname "%s" oifname "%s" masquerade
    }
}
`, table, bridge, bridge, bridge, bridge, bridge, bridge, bridge, wanIF, portList, bridge, wanIF, bridge, bridge, bridge, wanIF)
}

// VethNameForSandbox generates deterministic, instance-bound veth device names <= 15 chars (Linux IFNAMSIZ).
// Format: vh<slot>_<id8> (e.g. vh1_40f5569d, vh250_40f5569d).
// This guarantees that even if a slot is reused, teardown of an earlier sandbox cannot delete
// a newer sandbox's veth interface.
func VethNameForSandbox(slotIndex int, sandboxID string) (vethHost, vethGuest string) {
	cleanID := strings.TrimPrefix(sandboxID, "sbx_")
	cleanID = strings.ReplaceAll(cleanID, "-", "")
	if len(cleanID) > 8 {
		cleanID = cleanID[:8]
	} else if len(cleanID) == 0 {
		cleanID = "00000000"
	}

	vethHost = fmt.Sprintf("vh%d_%s", slotIndex, cleanID)
	vethGuest = fmt.Sprintf("vb%d_%s", slotIndex, cleanID)

	if len(vethHost) > 15 {
		vethHost = vethHost[:15]
	}
	if len(vethGuest) > 15 {
		vethGuest = vethGuest[:15]
	}
	return vethHost, vethGuest
}

// SetupSandboxNetwork provisions an isolated veth pair, attaches to the bridge with port isolation,
// and moves the guest interface into the sandbox's network namespace.
func SetupSandboxNetwork(ctx context.Context, bridge string, slotIndex int, sandboxID string, targetNetns string) (*SandboxNetwork, error) {
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

	vethHost, vethGuest := VethNameForSandbox(slotIndex, sandboxID)
	subnetOctet := (slotIndex - 1) / 240
	hostOctet := 10 + ((slotIndex - 1) % 240)
	boxIP := fmt.Sprintf("10.200.%d.%d/16", subnetOctet, hostOctet)

	// Clean up any stale interfaces for this specific instance
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
	if out, err := exec.CommandContext(ctx, "ip", "link", "set", vethHost, "type", "bridge_slave", "isolated", "on").CombinedOutput(); err != nil {
		_ = deleteSandboxInterface(ctx, vethHost)
		return nil, fmt.Errorf("enable bridge isolation: %s: %w", out, err)
	}

	// 4. Move guest interface into the sandbox network namespace
	if out, err := exec.CommandContext(ctx, "ip", "link", "set", vethGuest, "netns", targetNetns).CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("move %s to netns %s: %s (%w)", vethGuest, targetNetns, strings.TrimSpace(string(out)), err)
	}

	// 5-8. Configure guest interfaces in a single batched nsenter invocation (avoids fork storm)
	script := fmt.Sprintf(
		"ip link set %s name eth0 && ip addr add %s dev eth0 && ip link set eth0 up && ip link set lo up && ip route add default via 10.200.0.1",
		vethGuest, boxIP,
	)
	cmdGuest := exec.CommandContext(ctx, "nsenter", "--net="+targetNetns, "sh", "-c", script)
	if out, err := cmdGuest.CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		return nil, fmt.Errorf("configure sandbox netns %s interfaces: %s (%w)", targetNetns, strings.TrimSpace(string(out)), err)
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
func TeardownSandboxNetwork(ctx context.Context, slotIndex int, sandboxID string) error {
	if slotIndex <= 0 {
		return cleanupSandboxInterfaces(ctx, sandboxID)
	}
	vethHost, _ := VethNameForSandbox(slotIndex, sandboxID)
	return deleteSandboxInterface(ctx, vethHost)
}

func deleteSandboxInterface(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "ip", "link", "del", name).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Cannot find device") && !strings.Contains(string(out), "does not exist") {
		return fmt.Errorf("delete network interface %s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// CleanupSandboxInterfaces runs only before the gateway/control client start.
func CleanupSandboxInterfaces(ctx context.Context) error { return cleanupSandboxInterfaces(ctx, "") }

func cleanupSandboxInterfaces(ctx context.Context, sandboxID string) error {
	out, err := exec.CommandContext(ctx, "ip", "-j", "link", "show").Output()
	if err != nil {
		return fmt.Errorf("list residual network interfaces: %w", err)
	}
	var links []struct {
		Name string `json:"ifname"`
	}
	if err := json.Unmarshal(out, &links); err != nil {
		return err
	}
	var errs []error
	for _, link := range links {
		if !strings.HasPrefix(link.Name, "vh") {
			continue
		}
		part := strings.SplitN(strings.TrimPrefix(link.Name, "vh"), "_", 2)
		slot, err := strconv.Atoi(part[0])
		if err != nil || slot <= 0 {
			continue
		}
		if sandboxID != "" {
			expected, _ := VethNameForSandbox(slot, sandboxID)
			if link.Name != expected {
				continue
			}
		}
		if err := deleteSandboxInterface(ctx, link.Name); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
