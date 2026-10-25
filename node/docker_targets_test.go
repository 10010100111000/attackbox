package node

import (
	"attackbox/protocol"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The fake Engine exercises failure paths and request contracts without a daemon.
type targetEngineFixture struct {
	mu                                 sync.Mutex
	container                          *targetContainer
	creates, starts, deletes, pulls    int
	imagePresent, failPull, failDelete bool
	body                               map[string]any
}

func (f *targetEngineFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := r.URL.Path
	switch {
	case strings.HasPrefix(path, "/v1.47/networks/"):
		cfg := targetTestConfig().DockerTargets
		var n dockerNetwork
		n.Name = cfg.Name
		n.Driver = "bridge"
		n.Options = targetNetworkOptions(cfg)
		n.Labels = map[string]string{targetNetworkLabel: cfg.Name}
		n.IPAM.Driver = "default"
		n.IPAM.Config = append(n.IPAM.Config, struct{ Subnet, Gateway, IPRange string }{Subnet: cfg.SubnetCIDR, Gateway: cfg.GatewayIP})
		json.NewEncoder(w).Encode(n)
	case strings.HasPrefix(path, "/v1.47/images/") && r.Method == "GET":
		if !f.imagePresent {
			w.WriteHeader(404)
			return
		}
		fmt.Fprint(w, "{}")
	case path == "/v1.47/images/create":
		f.pulls++
		if f.failPull {
			fmt.Fprintln(w, `{"error":"registry secret must not escape"}`)
			return
		}
		f.imagePresent = true
		fmt.Fprintln(w, `{"status":"complete"}`)
	case path == "/v1.47/containers/create":
		f.creates++
		if err := json.NewDecoder(r.Body).Decode(&f.body); err != nil {
			w.WriteHeader(400)
			return
		}
		raw, _ := json.Marshal(f.body)
		var c targetContainer
		json.Unmarshal(raw, &c.Config)
		c.ID = "container-test-id"
		c.State.Status = "created"
		c.NetworkSettings.Networks = map[string]struct{ IPAddress string }{"attackbox-targets": {IPAddress: "10.201.0.2"}}
		f.container = &c
		w.WriteHeader(201)
		fmt.Fprint(w, `{"Id":"container-test-id"}`)
	case path == "/v1.47/containers/json":
		if f.container == nil {
			fmt.Fprint(w, "[]")
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{{"Id": f.container.ID, "Labels": f.container.Config.Labels, "State": f.container.State.Status}})
	case strings.HasSuffix(path, "/json"):
		if f.container == nil {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(f.container)
	case strings.HasSuffix(path, "/start"):
		f.starts++
		f.container.State.Running = true
		f.container.State.Status = "running"
		w.WriteHeader(204)
	case r.Method == "DELETE":
		if f.failDelete {
			w.WriteHeader(500)
			return
		}
		if r.URL.Query().Get("force") != "true" || r.URL.Query().Get("v") != "true" {
			w.WriteHeader(400)
			return
		}
		f.deletes++
		f.container = nil
		w.WriteHeader(204)
	default:
		w.WriteHeader(500)
	}
}

func newTargetEngineTest(t *testing.T) (*DockerTargetManager, *targetEngineFixture, *http.Client) {
	t.Helper()
	f := &targetEngineFixture{imagePresent: true}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	m, err := NewDockerTargetManager(targetTestConfig().DockerTargets, "test-node", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: rewriteTransport{base: srv.URL}, Timeout: time.Second}
	m.client = client
	return m, f, client
}

func targetTestRequest() protocol.TargetRequest {
	return protocol.TargetRequest{TargetID: "tgt_0123456789abcdef", OwnerID: "user-1", Image: "example/target:1.0", Ports: []protocol.TargetPort{{Port: 53, Protocol: "udp"}}}
}

func TestTargetStartIdempotenceOwnershipAndRestart(t *testing.T) {
	m, f, client := newTargetEngineTest(t)
	req := targetTestRequest()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		s, err := m.Start(ctx, req)
		if err != nil || s.State != "running" || s.Ready {
			t.Fatalf("start: %+v %v", s, err)
		}
	}
	if f.creates != 1 || f.starts != 1 {
		t.Fatal("duplicate container/start")
	}
	host := f.body["HostConfig"].(map[string]any)
	if host["Privileged"] != false || host["Memory"] != float64(512<<20) || host["NetworkMode"] != "attackbox-targets" {
		t.Fatalf("unexpected policy: %+v", host)
	}
	for _, key := range []string{"Binds", "Mounts", "PortBindings", "VolumesFrom"} {
		if _, ok := host[key]; ok {
			t.Fatalf("unexpected host option %s", key)
		}
	}
	other := req
	other.OwnerID = "user-2"
	if _, err := m.Stop(ctx, other); err == nil {
		t.Fatal("foreign owner stopped target")
	}
	other = req
	other.Image = "example/changed:2"
	if _, err := m.Start(ctx, other); err == nil {
		t.Fatal("immutable instance changed configuration")
	}
	restarted, err := NewDockerTargetManager(m.cfg, m.nodeID, m.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	restarted.client = client
	if _, err := restarted.Start(ctx, req); err != nil || f.creates != 1 {
		t.Fatalf("restart did not adopt container: %v", err)
	}
	snapshot := restarted.Inventory(ctx)
	if !snapshot.Available || len(snapshot.Instances) != 1 {
		t.Fatalf("inventory: %+v", snapshot)
	}
	if s, err := restarted.Stop(ctx, req); err != nil || s.State != "stopped" {
		t.Fatalf("stop: %+v %v", s, err)
	}
	if _, err := restarted.Stop(ctx, req); err != nil || f.deletes != 1 {
		t.Fatal("stop not idempotent")
	}
	again, err := NewDockerTargetManager(m.cfg, m.nodeID, m.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	again.client = client
	if _, err := again.Start(ctx, req); err == nil || f.creates != 1 {
		t.Fatal("late start resurrected stopped instance")
	}
}

func TestTargetPullAndCleanupFailuresRetainResponsibility(t *testing.T) {
	m, f, _ := newTargetEngineTest(t)
	req := targetTestRequest()
	ctx := context.Background()
	f.imagePresent = false
	f.failPull = true
	if _, err := m.Start(ctx, req); err == nil || err.Error() != "target_image_not_prepared" {
		t.Fatalf("pull failure not sanitized: %v", err)
	}
	if len(m.records) != 1 {
		t.Fatal("failed operation lost its intent")
	}
	f.failPull = false
	f.imagePresent = true
	if f.pulls != 0 {
		t.Fatal("learner start downloaded an image")
	}
	if _, err := m.Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	f.failDelete = true
	if s, err := m.Stop(ctx, req); err == nil || s.State != "cleanup_pending" {
		t.Fatalf("false stop success: %+v %v", s, err)
	}
	if _, err := m.Start(ctx, req); err == nil {
		t.Fatal("pending cleanup allowed recreation")
	}
	f.failDelete = false
	inventory := m.Inventory(ctx)
	if !inventory.Available || f.container != nil {
		t.Fatalf("cleanup not reconciled: %+v", inventory)
	}
}

func TestTargetNeverDeletesUnownedContainerAndReportsMissing(t *testing.T) {
	m, f, _ := newTargetEngineTest(t)
	req := targetTestRequest()
	ctx := context.Background()
	if _, err := m.Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	delete(f.container.Config.Labels, targetNodeLabel)
	if _, err := m.Stop(ctx, req); err == nil || f.deletes != 0 {
		t.Fatal("deleted unowned container")
	}
	// Recover label for cleanup, then use a different immutable ID for missing case.
	f.container.Config.Labels[targetNodeLabel] = m.nodeID
	if _, err := m.Stop(ctx, req); err != nil {
		t.Fatal(err)
	}
	req.TargetID = "tgt_1123456789abcdef"
	if _, err := m.Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	f.container = nil
	inventory := m.Inventory(ctx)
	if !inventory.Available || len(inventory.Instances) != 1 || inventory.Instances[0].State != "missing" {
		t.Fatalf("missing state lost: %+v", inventory)
	}
}

func TestTargetInventoryNeverWaitsForStartLock(t *testing.T) {
	m, _, _ := newTargetEngineTest(t)
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.Inventory(context.Background())
	if snapshot.Available || snapshot.Error != "target_operation_in_progress" {
		t.Fatalf("busy inventory claimed complete: %+v", snapshot)
	}
}

func TestTargetStateCorruptionAndIdentityValidation(t *testing.T) {
	m, _, _ := newTargetEngineTest(t)
	if err := os.WriteFile(filepath.Join(m.stateDir, "broken.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDockerTargetManager(m.cfg, m.nodeID, m.stateDir); err == nil {
		t.Fatal("corrupt intent silently ignored")
	}
	for _, mutate := range []func(*protocol.TargetRequest){
		func(r *protocol.TargetRequest) { r.TargetID = "../../escape" },
		func(r *protocol.TargetRequest) { r.OwnerID = "" },
		func(r *protocol.TargetRequest) { r.Image = "example/target" },
		func(r *protocol.TargetRequest) { r.Image = "example/target@sha256:bad" },
		func(r *protocol.TargetRequest) { r.Ports = []protocol.TargetPort{{Port: 0, Protocol: "tcp"}} },
	} {
		r := targetTestRequest()
		mutate(&r)
		if validateTargetRequest(r, true) == nil {
			t.Fatalf("invalid request accepted: %+v", r)
		}
	}
}

func TestTargetCommandGenerationAndDisabledMode(t *testing.T) {
	m, _, _ := newTargetEngineTest(t)
	c := &NodeControlClient{bootID: "current", targets: m}
	req := targetTestRequest()
	req.BootID = "previous"
	msg, _ := protocol.NewMessage(protocol.MsgTypeTargetStart, req)
	if r := c.executeTargetCommand(context.Background(), msg); r.Error != "worker_generation_changed" {
		t.Fatalf("stale generation accepted: %+v", r)
	}
	req.BootID = "current"
	msg, _ = protocol.NewMessage(protocol.MsgTypeTargetStart, req)
	if r := c.executeTargetCommand(context.Background(), msg); !r.Success {
		t.Fatalf("current command failed: %+v", r)
	}
	c.targets = nil
	if r := c.executeTargetCommand(context.Background(), msg); r.Error != "docker_targets_disabled" {
		t.Fatalf("disabled Worker pretended ready: %+v", r)
	}
}

func TestDockerStateDirectoryOutsideBtrfsTree(t *testing.T) {
	cfg := &NodeConfig{}
	cfg.Runtime.InstancesDir = "/srv/attackbox/instances"
	if got := targetStateDirectory(cfg); got != "/srv/attackbox/docker-target-state" {
		t.Fatalf("Docker intent overlaps Btrfs cleanup: %s", got)
	}
}

func TestDockerStateDirectoryCannotBeInsideBtrfs(t *testing.T) {
	root := t.TempDir()
	cfg := &NodeConfig{}
	cfg.Node.NodeID = "test-node"
	cfg.Network = targetTestConfig()
	cfg.Runtime.InstancesDir = filepath.Join(root, "instances")
	cfg.Network.DockerTargets.StateDir = filepath.Join(cfg.Runtime.InstancesDir, "docker-target-state")
	if _, err := newConfiguredTargetManager(cfg); err == nil {
		t.Fatal("Btrfs cleanup can see Docker intent directory")
	}
}

func TestStopBeforeStartBlocksDelayedCreation(t *testing.T) {
	m, f, client := newTargetEngineTest(t)
	req := targetTestRequest()
	if s, err := m.Stop(context.Background(), req); err != nil || s.State != "stopped" {
		t.Fatalf("stop unknown: %+v %v", s, err)
	}
	restarted, err := NewDockerTargetManager(m.cfg, m.nodeID, m.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	restarted.client = client
	if _, err := restarted.Start(context.Background(), req); err == nil || f.creates != 0 {
		t.Fatal("delayed start created a container after stop")
	}
}

// A deliberately stalled registry must not block stop/inspect or enqueue a
// second download. This exercises the separation rather than timing a fast pull.
func TestImagePreparationDoesNotBlockLifecycle(t *testing.T) {
	m, f, _ := newTargetEngineTest(t)
	f.imagePresent = false
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	original := m.client.Transport
	m.client.Transport = targetBlockingPullTransport{original, started, release}
	ctx := context.Background()
	state, err := m.ImageStatus(ctx, targetTestRequest().Image, true)
	if err != nil || state != "preparing" {
		t.Fatalf("prepare: %s %v", state, err)
	}
	<-started
	state, err = m.ImageStatus(ctx, targetTestRequest().Image, true)
	if err != nil || state != "preparing" {
		t.Fatalf("duplicate: %s %v", state, err)
	}
	done := make(chan error, 1)
	go func() { _, err := m.Stop(ctx, targetTestRequest()); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("download blocked stop")
	}
}

type targetBlockingPullTransport struct {
	next    http.RoundTripper
	started chan struct{}
	release chan struct{}
}

func (t targetBlockingPullTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/v1.47/images/create" {
		close(t.started)
		select {
		case <-t.release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	return t.next.RoundTrip(r)
}

func TestImagePreparationReportsOutcome(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			m, f, _ := newTargetEngineTest(t)
			f.imagePresent = false
			f.failPull = fail
			if state, err := m.ImageStatus(context.Background(), targetTestRequest().Image, false); err != nil || state != "missing" {
				t.Fatalf("initial %s %v", state, err)
			}
			if _, err := m.ImageStatus(context.Background(), targetTestRequest().Image, true); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for {
				state, err := m.ImageStatus(context.Background(), targetTestRequest().Image, false)
				if err != nil {
					t.Fatal(err)
				}
				if state != "preparing" {
					want := "ready"
					if fail {
						want = "failed"
					}
					if state != want {
						t.Fatalf("want %s got %s", want, state)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("preparation did not finish")
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}
