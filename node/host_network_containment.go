package node

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"attackbox/node/labnetwork"
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
	SlotIndex int                          `json:"slot_index"`
	IP        string                       `json:"ip"`
	VethHost  string                       `json:"veth_host"`
	VethGuest string                       `json:"veth_guest"`
	Namespace labnetwork.NamespaceIdentity `json:"namespace"`
}

// EnsureHostNetwork initializes the host bridge, IPv4 forwarding, NAT, and nftables containment baseline.
func EnsureHostNetwork(cfg NetworkConfig) error {
	if err := validateSandboxNetwork(cfg); err != nil {
		return err
	}
	table, err := firewallTableName(cfg.FirewallTable)
	if err != nil {
		return err
	}
	bridge := cfg.BridgeName
	gatewayIP := cfg.GatewayIP
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

	// All bridge/address/route operations use netlink, like the per-lab adapter.
	if err := ensureSandboxBridge(cfg, bridgeCIDR); err != nil {
		return err
	}
	wanIF := cfg.Interface
	if wanIF == "" {
		wanIF, err = detectDefaultWANInterface()
		if err != nil {
			return err
		}
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

// Sandbox eth0 containment is independent of per-instance lab networks.
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
