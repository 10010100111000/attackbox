package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func targetTestConfig() NetworkConfig {
	return NetworkConfig{Interface: "eth0", BridgeName: "br-attackbox", SubnetCIDR: "10.200.0.0/16", GatewayIP: "10.200.0.1", DockerTargets: DockerTargetNetworkConfig{Enabled: true, SocketPath: "/var/run/docker.sock", Name: "attackbox-targets", BridgeName: "br-ab-targets", SubnetCIDR: "10.201.0.0/16", GatewayIP: "10.201.0.1"}}
}

func TestTargetNetworkValidation(t *testing.T) {
	cfg := targetTestConfig()
	if err := validateTargetNetwork(cfg); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*NetworkConfig){
		func(c *NetworkConfig) {
			c.DockerTargets.SubnetCIDR = "10.200.128.0/24"
			c.DockerTargets.GatewayIP = "10.200.128.1"
		},
		func(c *NetworkConfig) { c.DockerTargets.BridgeName = c.BridgeName },
		func(c *NetworkConfig) { c.DockerTargets.BridgeName = "br-x\"; flush ruleset" },
		func(c *NetworkConfig) { c.DockerTargets.GatewayIP = "10.202.0.1" },
		func(c *NetworkConfig) { c.DockerTargets.GatewayIP = "10.201.255.255" },
		func(c *NetworkConfig) { c.DockerTargets.SocketPath = "tcp://remote" },
	} {
		c := cfg
		mutate(&c)
		if validateTargetNetwork(c) == nil {
			t.Fatalf("invalid configuration accepted: %+v", c)
		}
	}
}

func TestTargetFirewallDirectionsAndIsolation(t *testing.T) {
	cfg := targetTestConfig()
	rules := renderFirewallRules("attackbox", cfg.BridgeName, "eth0", "53, 80, 443", cfg)
	for _, want := range []string{
		`iifname "br-attackbox" oifname "br-ab-targets" ip saddr 10.200.0.0/16 ip daddr 10.201.0.0/16 accept`,
		`iifname "br-ab-targets" oifname "br-attackbox" ip saddr 10.201.0.0/16 ip daddr 10.200.0.0/16 accept`,
		`iifname "br-attackbox" oifname "br-attackbox" drop`,
		`iifname "br-ab-targets" drop`,
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Index(rules, `oifname "br-ab-targets" ip saddr`) > strings.LastIndex(rules, `iifname "br-attackbox" drop`) {
		t.Fatal("cross-bridge allow is unreachable")
	}
	if strings.Contains(rules, `iifname "br-ab-targets" masquerade`) {
		t.Fatal("targets must not be masqueraded")
	}
	if strings.Count(rules, "masquerade") != 2 {
		t.Fatal("unexpected NAT rule (one comment, one sandbox WAN rule expected)")
	}
	for _, r := range targetForwardingRules(cfg)[:2] {
		flat := strings.Join(r, " ")
		if strings.Contains(flat, "--dport") || !strings.Contains(flat, "-s ") || !strings.Contains(flat, "-d ") {
			t.Fatal("forwarding must preserve all lab ports and constrain both subnets")
		}
	}
}

func TestDockerNetworkCreateReuseAndConflict(t *testing.T) {
	cfg := targetTestConfig().DockerTargets
	var stored dockerNetwork
	exists := false
	creates := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if !exists {
				w.WriteHeader(404)
				return
			}
			json.NewEncoder(w).Encode(stored)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/v1.47/networks/create" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
			t.Error(err)
		}
		creates++
		exists = true
		w.WriteHeader(201)
	}))
	defer srv.Close()
	client := &http.Client{Transport: rewriteTransport{base: srv.URL}}
	for i := 0; i < 2; i++ {
		if err := ensureDockerNetwork(context.Background(), client, cfg); err != nil {
			t.Fatal(err)
		}
	}
	if creates != 1 {
		t.Fatal("network was recreated")
	}
	stored.Options["com.docker.network.bridge.enable_ip_masquerade"] = "true"
	if ensureDockerNetwork(context.Background(), client, cfg) == nil {
		t.Fatal("conflicting NAT network accepted")
	}
	delete(stored.Labels, targetNetworkLabel)
	if ensureDockerNetwork(context.Background(), client, cfg) == nil || creates != 1 {
		t.Fatal("unowned network adopted or recreated")
	}
}

type rewriteTransport struct{ base string }

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *req.URL
	clone.URL = &u
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(r.base, "http://")
	return http.DefaultTransport.RoundTrip(clone)
}

func TestDockerForwardingIdempotentAndBackendFailure(t *testing.T) {
	inserts := 0
	installed := map[string]bool{}
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-S FORWARD") {
			return []byte("-A FORWARD -j DOCKER-USER\n"), nil
		}
		if strings.Contains(joined, "-C DOCKER-USER") {
			if installed[joined] {
				return nil, nil
			}
			return nil, fmt.Errorf("absent")
		}
		inserts++
		installed[strings.Replace(joined, "-I DOCKER-USER 1", "-C DOCKER-USER", 1)] = true
		return nil, nil
	}
	for i := 0; i < 2; i++ {
		if err := ensureTargetForwarding(context.Background(), targetTestConfig(), run); err != nil {
			t.Fatal(err)
		}
	}
	if inserts != 5 {
		t.Fatalf("duplicate forwarding rules: %d", inserts)
	}
	bad := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("unsupported backend")
	}
	if ensureTargetForwarding(context.Background(), targetTestConfig(), bad) == nil {
		t.Fatal("unsupported backend accepted")
	}
	failInsert := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "-S FORWARD") {
			return []byte("-A FORWARD -j DOCKER-USER"), nil
		}
		return nil, fmt.Errorf("permission denied")
	}
	if ensureTargetForwarding(context.Background(), targetTestConfig(), failInsert) == nil {
		t.Fatal("failed installation ignored")
	}
}

// Opt-in real local Docker API acceptance. It never edits host firewall rules.
func TestDockerTargetNetworkRealEngine(t *testing.T) {
	socket := os.Getenv("ATTACKBOX_TEST_DOCKER_SOCKET")
	if socket == "" {
		t.Skip("set ATTACKBOX_TEST_DOCKER_SOCKET for local Docker network acceptance")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	cfg := targetTestConfig().DockerTargets
	suffix := fmt.Sprintf("%x", time.Now().UnixNano())
	cfg.Name = "ab-net-test-" + suffix
	cfg.BridgeName = "abt" + suffix[len(suffix)-9:]
	cfg.SubnetCIDR = "172.30.231.0/24"
	cfg.GatewayIP = "172.30.231.1"
	// Only delete a network carrying this test's exact ownership label.
	defer func() {
		var n dockerNetwork
		if _, err := dockerNetworkRequest(context.Background(), client, "GET", "/networks/"+cfg.Name, nil, &n); err == nil && n.Labels[targetNetworkLabel] == cfg.Name {
			if _, err := dockerNetworkRequest(context.Background(), client, "DELETE", "/networks/"+cfg.Name, nil, nil); err != nil {
				t.Error(err)
			}
		}
	}()
	for i := 0; i < 2; i++ {
		if err := ensureDockerNetwork(context.Background(), client, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDockerNetworkConfigDefaultsAndEnabledValidation(t *testing.T) {
	path := t.TempDir() + "/node.yaml"
	if err := os.WriteFile(path, []byte("network:\n  docker_targets:\n    enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadNodeConfig(path)
	if err != nil || cfg.Network.DockerTargets.BridgeName != "br-ab-targets" {
		t.Fatalf("defaults: %v, %+v", err, cfg)
	}
	if err := os.WriteFile(path, []byte("network:\n  docker_targets:\n    enabled: true\n    subnet_cidr: 10.200.0.0/16\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNodeConfig(path); err == nil {
		t.Fatal("overlapping configured network accepted")
	}
	cfg, err = LoadNodeConfig("../configs/node.yaml")
	if err != nil || cfg.Network.DockerTargets.Enabled {
		t.Fatalf("sample config should preserve disabled default: %v", err)
	}
}

func TestTargetInternetBlockPrecedesEstablishedForwarding(t *testing.T) {
	cfg := targetTestConfig()
	rules := renderFirewallRules("attackbox", cfg.BridgeName, "eth0", "53, 80, 443", cfg)
	forward := strings.SplitN(rules, "chain forward {", 2)[1]
	block := strings.Index(forward, `iifname "br-ab-targets" drop`)
	established := strings.Index(forward, "ct state established,related accept")
	if block < 0 || established < 0 || block > established {
		t.Fatal("pre-existing connections can bypass target egress block")
	}
	if strings.Contains(rules, "flush ruleset") || strings.Contains(rules, "policy drop") {
		t.Fatal("unrelated host forwarding changed")
	}
}
