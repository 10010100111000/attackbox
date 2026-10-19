package network

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"attackbox/internal/worker/config"
)

// EnsureHostNetwork initializes the host bridge, IPv4 forwarding, NAT, and nftables containment baseline.
func EnsureHostNetwork(cfg config.NetworkConfig) error {
	bridge := cfg.BridgeName
	if bridge == "" {
		bridge = "br0"
	}
	bridgeIP := cfg.BridgeIP
	if bridgeIP == "" {
		bridgeIP = "10.200.0.1/16"
	}
	targetBridge := cfg.TargetBridge
	if targetBridge == "" {
		targetBridge = "docker0"
	}

	// 1. Enable IPv4 kernel forwarding
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644)
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	// 2. Idempotently create bridge device
	if err := exec.Command("ip", "link", "show", bridge).Run(); err != nil {
		_ = exec.Command("ip", "link", "add", bridge, "type", "bridge").Run()
		_ = exec.Command("ip", "addr", "add", bridgeIP, "dev", bridge).Run()
		_ = exec.Command("ip", "link", "set", bridge, "up").Run()
	} else {
		// Ensure bridge is UP
		_ = exec.Command("ip", "link", "set", bridge, "up").Run()
	}

	// 3. Detect default WAN interface if not specified
	wanIF := cfg.WANInterface
	if wanIF == "" {
		wanIF = detectDefaultWANInterface()
	}

	// 4. Build allowed WAN egress port list
	allowedPorts := cfg.EgressAllowedPorts
	if len(allowedPorts) == 0 {
		allowedPorts = []int{53, 80, 443}
	}
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

        # Inter-sandbox and sandbox-to-target completely allowed (full ports & protocols)
        iifname "%s" oifname "%s" accept
        iifname "%s" oifname "%s" accept
        iifname "%s" oifname "podman*" accept
        iifname "%s" oifname "tun*" accept
        iifname "%s" oifname "wg*" accept

        # DNS resolution pass-through
        iifname "%s" ip daddr { 1.1.1.1, 8.8.8.8, 8.8.4.4, 9.9.9.9 } udp dport 53 accept
        iifname "%s" ip daddr { 1.1.1.1, 8.8.8.8, 8.8.4.4, 9.9.9.9 } tcp dport 53 accept

        # WAN egress: strictly whitelist allowed ports, reject all others
        iifname "%s" oifname "%s" tcp dport { %s } accept
        iifname "%s" oifname "%s" reject
    }
    chain input {
        type filter hook input priority 0;
        iifname "%s" ct state established,related accept
        iifname "%s" icmp type echo-request accept
        # Block sandboxes from accessing host-local services (e.g. host SSH 22, host databases)
        iifname "%s" reject
    }
}
`,
		bridge, bridge,
		bridge, targetBridge,
		bridge,
		bridge,
		bridge,
		bridge,
		bridge,
		bridge, wanIF, portList,
		bridge, wanIF,
		bridge,
		bridge,
		bridge,
	)

	_ = exec.Command("nft", "delete", "table", "inet", "attackbox").Run()
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewReader([]byte(nftRules))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apply nftables: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	// 6. Idempotently maintain NAT MASQUERADE for egress traffic
	// Determine subnet for iptables NAT rule
	subnet := "10.200.0.0/16"
	if strings.HasSuffix(bridgeIP, "/24") {
		subnet = "10.200.0.0/24"
	}
	if err := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", subnet, "-o", wanIF, "-j", "MASQUERADE").Run(); err != nil {
		_ = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", subnet, "-o", wanIF, "-j", "MASQUERADE").Run()
	}

	return nil
}

// detectDefaultWANInterface inspects routing table to locate the primary egress network device.
func detectDefaultWANInterface() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "eth0"
	}

	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return "eth0"
}
