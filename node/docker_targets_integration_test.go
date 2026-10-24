package node

import (
	"attackbox/protocol"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// This opts into a local daemon only and cleans up only its own labelled data.
// See worker-docker-target-runtime.md for the no-network fixture image build.
func TestDockerTargetLifecycleRealEngine(t *testing.T) {
	socket, image := os.Getenv("ATTACKBOX_TEST_DOCKER_SOCKET"), os.Getenv("ATTACKBOX_TEST_TARGET_IMAGE")
	if socket == "" || image == "" {
		t.Skip("set ATTACKBOX_TEST_DOCKER_SOCKET and ATTACKBOX_TEST_TARGET_IMAGE")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	cfg := targetTestConfig().DockerTargets
	suffix := fmt.Sprintf("%016x", time.Now().UnixNano())
	cfg.SocketPath = socket
	cfg.Name = "ab-life-test-" + suffix
	cfg.BridgeName = "abl" + suffix[len(suffix)-9:]
	cfg.SubnetCIDR = "172.30.232.0/24"
	cfg.GatewayIP = "172.30.232.1"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ensureDockerNetwork(ctx, client, cfg); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := dockerNetworkRequest(context.Background(), client, "DELETE", "/networks/"+cfg.Name, nil, nil); err != nil {
			t.Error(err)
		}
	}()
	m, err := NewDockerTargetManager(cfg, "test-"+suffix, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	req := protocol.TargetRequest{TargetID: "tgt_" + suffix, OwnerID: "acceptance-user", Image: image, Ports: []protocol.TargetPort{{Port: 8080, Protocol: "tcp"}}}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := m.Stop(cleanupCtx, req); err != nil {
			t.Error(err)
		}
	}()
	first, err := m.Start(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContainerID == "" || first.State != "running" {
		t.Fatalf("not running: %+v", first)
	}
	second, err := m.Start(ctx, req)
	if err != nil || second.ContainerID != first.ContainerID {
		t.Fatalf("duplicate start: %+v %v", second, err)
	}
	restarted, err := NewDockerTargetManager(cfg, m.nodeID, m.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(12 * time.Second)
	var current protocol.TargetSummary
	for time.Now().Before(deadline) {
		current, err = restarted.Inspect(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if current.Ready {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !current.Ready || current.ContainerID != first.ContainerID || current.IP == "" {
		t.Fatalf("restart/readiness: %+v", current)
	}
	if s, err := restarted.Stop(ctx, req); err != nil || s.State != "stopped" {
		t.Fatalf("stop: %+v %v", s, err)
	}
	if _, err := restarted.Start(ctx, req); err == nil {
		t.Fatal("late start recreated stopped target")
	}
	if _, err := restarted.Stop(ctx, req); err != nil {
		t.Fatal(err)
	}
}
