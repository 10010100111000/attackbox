package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// DockerTargetNetworkConfig is opt-in until Docker is installed on the node.
// Network and bridge names must remain stable while containers use the network.
type DockerTargetNetworkConfig struct {
	Enabled    bool   `yaml:"enabled"`
	StateDir   string `yaml:"state_dir"`
	SocketPath string `yaml:"socket_path"`
	Name       string `yaml:"name"`
	BridgeName string `yaml:"bridge_name"`
	SubnetCIDR string `yaml:"subnet_cidr"`
	GatewayIP  string `yaml:"gateway_ip"`
}

const targetNetworkLabel = "io.attackbox.network"

type dockerNetwork struct {
	Name       string
	Driver     string
	Internal   bool
	EnableIPv6 bool
	Labels     map[string]string
	Options    map[string]string
	IPAM       struct {
		Driver string
		Config []struct{ Subnet, Gateway, IPRange string }
	}
}

func validateTargetNetwork(cfg NetworkConfig) error {
	t := cfg.DockerTargets
	if !t.Enabled {
		return nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`).MatchString(t.Name) || t.Name == "bridge" || t.Name == "host" || t.Name == "none" {
		return fmt.Errorf("invalid Docker target network name")
	}
	validBridge := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,14}$`)
	if !validBridge.MatchString(t.BridgeName) || !validBridge.MatchString(cfg.BridgeName) || t.BridgeName == cfg.BridgeName {
		return fmt.Errorf("Docker and sandbox bridges must have distinct valid names")
	}
	a, err := netip.ParsePrefix(cfg.SubnetCIDR)
	if err != nil || !a.Addr().Is4() {
		return fmt.Errorf("invalid sandbox subnet")
	}
	b, err := netip.ParsePrefix(t.SubnetCIDR)
	if err != nil || !b.Addr().Is4() || b != b.Masked() || b.Bits() > 29 {
		return fmt.Errorf("Docker target subnet must be canonical IPv4 /29 or larger")
	}
	if a.Overlaps(b) {
		return fmt.Errorf("Docker and sandbox subnets overlap")
	}
	gateway, err := netip.ParseAddr(t.GatewayIP)
	if err != nil || !b.Contains(gateway) || gateway == b.Addr() || !b.Contains(gateway.Next()) {
		return fmt.Errorf("invalid Docker target gateway")
	}
	if !strings.HasPrefix(t.SocketPath, "/") {
		return fmt.Errorf("Docker socket_path must be an absolute local path")
	}
	return nil
}

func targetNetworkOptions(t DockerTargetNetworkConfig) map[string]string {
	return map[string]string{
		"com.docker.network.bridge.name":                 t.BridgeName,
		"com.docker.network.bridge.enable_ip_masquerade": "false",
		"com.docker.network.bridge.enable_icc":           "true",
		// Docker 27+ supports routed access to unpublished ports in this mode.
		// Host firewall rules below constrain forwarding to our sandbox subnet.
		"com.docker.network.bridge.gateway_mode_ipv4": "nat-unprotected",
	}
}

func checkTargetNetwork(n dockerNetwork, t DockerTargetNetworkConfig) error {
	if n.Name != t.Name || n.Driver != "bridge" || n.Internal || n.EnableIPv6 || n.Labels[targetNetworkLabel] != t.Name {
		return fmt.Errorf("Docker network is not an owned IPv4 target bridge")
	}
	if n.IPAM.Driver != "default" || len(n.IPAM.Config) != 1 || n.IPAM.Config[0].Subnet != t.SubnetCIDR || n.IPAM.Config[0].Gateway != t.GatewayIP || n.IPAM.Config[0].IPRange != "" {
		return fmt.Errorf("Docker target network IPAM conflicts with configuration")
	}
	for k, v := range targetNetworkOptions(t) {
		if n.Options[k] != v {
			return fmt.Errorf("Docker target network option %s conflicts with configuration", k)
		}
	}
	return nil
}

func dockerNetworkRequest(ctx context.Context, client *http.Client, method, path string, payload any, result any) (int, error) {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker/v1.47"+path, &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("local Docker API unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("Docker network API returned HTTP %d", resp.StatusCode)
	}
	if result != nil {
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
	}
	return resp.StatusCode, err
}

func ensureDockerNetwork(ctx context.Context, client *http.Client, t DockerTargetNetworkConfig) error {
	path := "/networks/" + url.PathEscape(t.Name)
	var n dockerNetwork
	code, err := dockerNetworkRequest(ctx, client, "GET", path, nil, &n)
	if code == http.StatusNotFound {
		if _, err := net.InterfaceByName(t.BridgeName); err == nil {
			return fmt.Errorf("refusing to adopt existing bridge %s without an owned Docker network", t.BridgeName)
		}
		payload := map[string]any{"Name": t.Name, "Driver": "bridge", "EnableIPv6": false, "Options": targetNetworkOptions(t), "Labels": map[string]string{targetNetworkLabel: t.Name}, "IPAM": map[string]any{"Driver": "default", "Config": []map[string]string{{"Subnet": t.SubnetCIDR, "Gateway": t.GatewayIP}}}}
		_, err = dockerNetworkRequest(ctx, client, "POST", "/networks/create", payload, nil)
		if err != nil {
			return err
		}
		_, err = dockerNetworkRequest(ctx, client, "GET", path, nil, &n)
	}
	if err != nil {
		return err
	}
	return checkTargetNetwork(n, t)
}

type networkCommand func(context.Context, string, ...string) ([]byte, error)

func runNetworkCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// Docker's iptables backend calls DOCKER-USER before its isolation/default-drop
// rules. Native Docker nftables backend needs a separate adapter and is rejected.
func ensureTargetForwarding(ctx context.Context, cfg NetworkConfig, run networkCommand) error {
	out, err := run(ctx, "iptables", "-w", "5", "-S", "FORWARD")
	if err != nil || !strings.Contains(string(out), "-A FORWARD -j DOCKER-USER") {
		return fmt.Errorf("Docker iptables DOCKER-USER forwarding hook is required (native nftables backend is not supported yet)")
	}
	for _, rule := range targetForwardingRules(cfg) {
		args := append([]string{"-w", "5", "-C", "DOCKER-USER"}, rule...)
		if _, err := run(ctx, "iptables", args...); err == nil {
			continue
		}
		args = append([]string{"-w", "5", "-I", "DOCKER-USER", "1"}, rule...)
		if _, err := run(ctx, "iptables", args...); err != nil {
			return fmt.Errorf("cannot install target forwarding rule: %w", err)
		}
	}
	return nil
}

func targetForwardingRules(cfg NetworkConfig) [][]string {
	t := cfg.DockerTargets
	wan := cfg.Interface
	if wan == "" {
		wan = detectDefaultWANInterface()
	}
	return [][]string{
		{"-i", cfg.BridgeName, "-o", t.BridgeName, "-s", cfg.SubnetCIDR, "-d", t.SubnetCIDR, "-m", "comment", "--comment", "attackbox-target-forward", "-j", "ACCEPT"},
		{"-i", t.BridgeName, "-o", cfg.BridgeName, "-s", t.SubnetCIDR, "-d", cfg.SubnetCIDR, "-m", "comment", "--comment", "attackbox-target-forward", "-j", "ACCEPT"},
		// Docker may set the host FORWARD policy to DROP. Preserve the existing
		// sandbox WAN policy explicitly, rather than changing that global policy.
		{"-i", cfg.BridgeName, "-o", wan, "-s", cfg.SubnetCIDR, "-p", "tcp", "-m", "multiport", "--dports", "53,80,443", "-m", "comment", "--comment", "attackbox-sandbox-wan", "-j", "ACCEPT"},
		{"-i", cfg.BridgeName, "-o", wan, "-s", cfg.SubnetCIDR, "-p", "udp", "--dport", "53", "-m", "comment", "--comment", "attackbox-sandbox-wan", "-j", "ACCEPT"},
		{"-i", wan, "-o", cfg.BridgeName, "-d", cfg.SubnetCIDR, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-m", "comment", "--comment", "attackbox-sandbox-wan-reply", "-j", "ACCEPT"},
	}
}

func ensureDockerTargetNetwork(ctx context.Context, cfg NetworkConfig) error {
	if err := validateTargetNetwork(cfg); err != nil {
		return err
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", cfg.DockerTargets.SocketPath)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	// Check Docker's existing forwarding hook once and install the narrowly
	// scoped exceptions. Do not change host routes or the global FORWARD policy.
	if err := ensureDockerNetwork(ctx, client, cfg.DockerTargets); err != nil {
		return err
	}
	return ensureTargetForwarding(ctx, cfg, runNetworkCommand)
}
