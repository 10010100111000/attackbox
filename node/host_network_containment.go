package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NetworkConfig holds network virtualization settings for sandboxes.
type NetworkConfig struct {
	FirewallTable string                    `yaml:"firewall_table"`
	BridgeName    string                    `yaml:"bridge_name"`
	GatewayIP     string                    `yaml:"gateway_ip"`
	SubnetCIDR    string                    `yaml:"subnet_cidr"`
	Interface     string                    `yaml:"egress_interface"`
	DockerTargets DockerTargetNetworkConfig `yaml:"docker_targets"`
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
	if err := validateTargetNetwork(cfg); err != nil {
		return err
	}
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
	subnet, err := netip.ParsePrefix(cfg.SubnetCIDR)
	if err != nil || !subnet.Addr().Is4() {
		return fmt.Errorf("network subnet_cidr must be an IPv4 prefix")
	}
	gateway, err := netip.ParseAddr(gatewayIP)
	if err != nil || !subnet.Contains(gateway) {
		return fmt.Errorf("network gateway_ip must belong to subnet_cidr")
	}
	bridgeCIDR := fmt.Sprintf("%s/%d", gateway, subnet.Bits())

	// 1. Enable routing. Do not silently continue with an unusable host network.
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644); err != nil {
		return fmt.Errorf("enable IPv4 forwarding: %w", err)
	}

	// 2. Create our bridge, or verify the existing bridge has the configured address.
	out, lookupErr := exec.Command("ip", "-j", "address", "show", "dev", bridge).Output()
	if lookupErr != nil {
		for _, args := range [][]string{
			{"link", "add", bridge, "type", "bridge"},
			{"addr", "add", bridgeCIDR, "dev", bridge},
			{"link", "set", bridge, "up"},
		} {
			if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
				return fmt.Errorf("initialize sandbox bridge: %s: %w", strings.TrimSpace(string(out)), err)
			}
		}
	} else {
		var links []struct {
			Addresses []struct {
				Local     string `json:"local"`
				PrefixLen int    `json:"prefixlen"`
			} `json:"addr_info"`
		}
		if err := json.Unmarshal(out, &links); err != nil {
			return fmt.Errorf("inspect sandbox bridge: %w", err)
		}
		matches := false
		for _, link := range links {
			for _, address := range link.Addresses {
				if address.Local == gatewayIP && address.PrefixLen == subnet.Bits() {
					matches = true
				}
			}
		}
		if !matches {
			return fmt.Errorf("existing sandbox bridge %s does not have configured address %s", bridge, bridgeCIDR)
		}
		if out, err := exec.Command("ip", "link", "set", bridge, "up").CombinedOutput(); err != nil {
			return fmt.Errorf("activate sandbox bridge: %s: %w", strings.TrimSpace(string(out)), err)
		}
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
	// - forward chain: Enforces same-bridge isolation and whitelisted internet egress
	// - postrouting chain: Masquerades authorized egress to the external network
	nftRules := renderFirewallRules(table, bridge, wanIF, portList)
	if cfg.DockerTargets.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ensureDockerTargetNetwork(ctx, cfg); err != nil {
			return err
		}
		nftRules = renderFirewallRules(table, bridge, wanIF, portList, cfg)
	}
	// Replace only our own table, atomically, without accumulating duplicate rules.
	if exec.Command("nft", "list", "table", "inet", table).Run() == nil {
		nftRules = "delete table inet " + table + "\n" + nftRules
	}

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

// One template owns the Worker firewall; Docker only needs matching exceptions
// in its existing forwarding hook. No extra bridge or routing layer is created.
func renderFirewallRules(table, bridge, wanIF, portList string, configs ...NetworkConfig) string {
	targetInput, targetForward := "", ""
	if len(configs) > 0 && configs[0].DockerTargets.Enabled {
		cfg := configs[0]
		t := cfg.DockerTargets
		targetInput = fmt.Sprintf("        iifname %q drop\n", t.BridgeName)
		targetForward = fmt.Sprintf(`        # Existing connected routes carry lab traffic; only unblock forwarding.
        iifname %q oifname %q ip saddr %s ip daddr %s accept
        iifname %q oifname %q ip saddr %s ip daddr %s accept
        # Block target egress even for connections established before this policy.
        iifname %q drop
        oifname %q drop
`, bridge, t.BridgeName, cfg.SubnetCIDR, t.SubnetCIDR, t.BridgeName, bridge, t.SubnetCIDR, cfg.SubnetCIDR, t.BridgeName, t.BridgeName)
	}

	return fmt.Sprintf(`
table inet %s {
    chain input {
        type filter hook input priority 0; policy accept;
        ct state established,related accept

%s
        # Allow container to query host gateway DNS (53) and ping (icmp echo-request)
        iifname "%s" udp dport 53 accept
        iifname "%s" tcp dport 53 accept
        iifname "%s" icmp type echo-request accept

        # Reject all other inbound traffic from sandboxes to host control plane / services
        iifname "%s" drop
    }

    chain forward {
        type filter hook forward priority 0; policy accept;
%s
        ct state established,related accept

        # Preserve sandbox peer isolation.
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
`, table, targetInput, bridge, bridge, bridge, bridge, targetForward, bridge, bridge, bridge, wanIF, portList, bridge, wanIF, bridge, bridge, bridge, wanIF)
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

// SetupSandboxNetwork uses the default lab subnet for compatibility.
// New callers should use SetupSandboxNetworkWithConfig.

func SetupSandboxNetwork(ctx context.Context, bridge string, slotIndex int, sandboxID string, targetNetns string) (*SandboxNetwork, error) {
	return SetupSandboxNetworkWithConfig(ctx, NetworkConfig{BridgeName: bridge, GatewayIP: "10.200.0.1", SubnetCIDR: "10.200.0.0/16"}, slotIndex, sandboxID, targetNetns)
}

// SetupSandboxNetworkWithConfig attaches a sandbox to the shared lab bridge.
func SetupSandboxNetworkWithConfig(ctx context.Context, cfg NetworkConfig, slotIndex int, sandboxID string, targetNetns string) (*SandboxNetwork, error) {
	bridge := cfg.BridgeName
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
	boxIP, err := sandboxAddress(cfg, slotIndex)
	if err != nil {
		return nil, err
	}

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

	// 3. Preserve isolation between sandbox bridge ports.
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
		"ip link set %s name eth0 && ip addr add %s dev eth0 && ip link set eth0 up && ip link set lo up && ip route add default via %s",
		vethGuest, boxIP, cfg.GatewayIP,
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

// sandboxAddress preserves the existing 240 slots per /24 allocation. Slots
// use offsets 10..249 and 266..275; Docker IPAM must not overlap them.
func sandboxAddress(cfg NetworkConfig, slot int) (string, error) {
	subnet, err := netip.ParsePrefix(cfg.SubnetCIDR)
	if err != nil || !subnet.Addr().Is4() || subnet.Bits() > 23 {
		return "", fmt.Errorf("sandbox subnet must be IPv4 /23 or larger")
	}
	subnet = subnet.Masked()
	gateway, err := netip.ParseAddr(cfg.GatewayIP)
	if err != nil || !subnet.Contains(gateway) {
		return "", fmt.Errorf("invalid sandbox gateway")
	}
	if slot < 1 || slot > 250 {
		return "", fmt.Errorf("sandbox slot must be between 1 and 250")
	}
	base := subnet.Addr().As4()
	offset := uint32((slot-1)/240)*256 + uint32(10+(slot-1)%240)
	raw := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	raw += offset
	address := netip.AddrFrom4([4]byte{byte(raw >> 24), byte(raw >> 16), byte(raw >> 8), byte(raw)})
	if address == gateway {
		return "", fmt.Errorf("sandbox address conflicts with gateway")
	}
	return fmt.Sprintf("%s/%d", address, subnet.Bits()), nil
}
