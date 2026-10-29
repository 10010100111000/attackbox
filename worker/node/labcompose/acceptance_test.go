package labcompose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"attackbox/node/labnetwork"
)

// Test-only network provider: real Docker internal networks, no sandbox wiring
// or host firewall changes. Full veth/INPUT acceptance remains in labnetwork.
type engineTestNetworks struct {
	*fakeNetworks
	cli  *CLI
	node string
}

func (n *engineTestNetworks) CreateNetwork(ctx context.Context, s labnetwork.NetworkSpec) (labnetwork.Network, error) {
	id := s.TargetID + "/" + s.Key
	n.mu.Lock()
	old, exists := n.nets[id]
	n.mu.Unlock()
	if exists {
		return old, nil
	}
	name := "ab_test_" + digest([]byte(n.node + "/" + id))[:24]
	b, err := n.cli.run(ctx, "network", "create", "--internal", "--driver", "bridge", "--label", "io.attackbox.fixture="+n.node, name)
	if err != nil {
		return labnetwork.Network{}, err
	}
	v := labnetwork.Network{ID: id, Spec: s, Name: name, DockerID: strings.TrimSpace(string(b)), Desired: true}
	n.mu.Lock()
	n.nets[id] = v
	n.mu.Unlock()
	return v, nil
}
func (n *engineTestNetworks) ReleaseNetwork(ctx context.Context, id string) error {
	n.mu.Lock()
	v, ok := n.nets[id]
	n.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown fixture network")
	}
	if v.Cleaned {
		return nil
	}
	b, err := n.cli.run(ctx, "network", "inspect", v.DockerID)
	if err != nil {
		return err
	}
	var networks []struct {
		ID     string `json:"Id"`
		Labels map[string]string
	}
	if json.Unmarshal(b, &networks) != nil || len(networks) != 1 || networks[0].ID != v.DockerID || networks[0].Labels["io.attackbox.fixture"] != n.node {
		return fmt.Errorf("fixture network ownership mismatch")
	}
	if _, err := n.cli.run(ctx, "network", "rm", v.DockerID); err != nil {
		return err
	}
	n.mu.Lock()
	v.Desired = false
	v.Cleaned = true
	n.nets[id] = v
	n.mu.Unlock()
	return nil
}

func TestComposeRealEngineLifecycle(t *testing.T) {
	if os.Getenv("ATTACKBOX_TEST_COMPOSE_ENGINE") != "1" {
		t.Skip("opt in with local Docker socket and offline fixture image")
	}
	socket, image := os.Getenv("ATTACKBOX_TEST_DOCKER_SOCKET"), os.Getenv("ATTACKBOX_TEST_LAB_IMAGE")
	if socket == "" || image == "" {
		t.Fatal("explicit socket and offline fixture image required")
	}
	cli, err := NewCLI(CLIConfig{DockerSocket: socket})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	node := "fixture-" + digest([]byte(time.Now().String()))[:12]
	nets := &engineTestNetworks{fakeNetworks: &fakeNetworks{nets: map[string]labnetwork.Network{}, attachments: map[string]labnetwork.Attachment{}}, cli: cli, node: node}
	cfg := Config{NodeID: node, StateDir: filepath.Join(t.TempDir(), "runtime"), Engine: cli, Networks: nets}
	m, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var requests []Request
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, r := range requests {
			if _, err := m.Stop(cleanupCtx, r.ID); err != nil {
				t.Error("fixture cleanup pending", r.ID, err)
			}
		}
		m.Close()
	})
	dir, err := filepath.Abs("../testdata/compose-lab")
	if err != nil {
		t.Fatal(err)
	}
	d, err := m.Prepare(ctx, Bundle{Directory: dir, File: "compose.yml", Variables: map[string]string{"LAB_IMAGE": image}, EntryService: "web", EntryPort: 8080})
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		r := req(d, "test_"+owner+"_"+node, owner)
		requests = append(requests, r)
		if _, err := m.Start(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	waitReady := func(r Request) Status {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			s, err := m.Inspect(ctx, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if s.Ready {
				return s
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("project never became healthy")
		return Status{}
	}
	for _, r := range requests {
		status := waitReady(r)
		if status.State != "running" || status.Attachment != "not_connected" {
			t.Fatal(status)
		}
	}
	containerID := func(r Request) string {
		t.Helper()
		e, err := m.get(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		items, err := cli.inventory(ctx, e.snapshot())
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range items {
			if c.Config.Labels["com.docker.compose.service"] == "web" {
				return c.ID
			}
		}
		t.Fatal("web missing")
		return ""
	}
	a, b := containerID(requests[0]), containerID(requests[1])
	bindings, err := cli.run(ctx, "container", "inspect", "--format", "{{json .HostConfig.PortBindings}}", a)
	if err != nil {
		t.Fatal(err)
	}
	var published map[string]any
	if json.Unmarshal(bindings, &published) != nil || len(published) != 0 {
		t.Fatal("host port mapping was not removed")
	}
	if a == b {
		t.Fatal("shared container")
	}
	if _, err := cli.run(ctx, "exec", a, "/fixture", "write", "/data/marker", "alice-only"); err != nil {
		t.Fatal(err)
	}
	if data, err := cli.run(ctx, "exec", a, "/fixture", "read", "/data/marker"); err != nil || string(data) != "alice-only" {
		t.Fatal("own data missing", err)
	}
	if _, err := cli.run(ctx, "exec", b, "/fixture", "read", "/data/marker"); err == nil {
		t.Fatal("cross-user volume shared")
	}
	if data, err := cli.run(ctx, "exec", b, "/fixture", "read", "/data/seed"); err != nil || string(data) != "initialized" {
		t.Fatal("initialization dependency failed", err)
	}
	if _, err := m.Start(ctx, requests[0]); err != nil || containerID(requests[0]) != a {
		t.Fatal("repeat start recreated container", err)
	}
	cfg = m.cfg
	m.Close()
	reopened, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m = reopened
	if s, err := m.Start(ctx, requests[0]); err != nil || s.State != "running" || containerID(requests[0]) != a {
		t.Fatal("restart lost instance", s, err)
	}
	m.clock = func() time.Time { return requests[1].ExpiresAt.Add(time.Second) }
	if err := m.Reap(ctx); err != nil {
		t.Fatal(err)
	}
	for _, r := range requests {
		if s, err := m.Inspect(ctx, r.ID); err != nil || s.State != "stopped" {
			t.Fatal(s, err)
		}
	}
	for _, v := range object(d.Definition.Document["services"]) {
		if err := cli.ImagePresent(ctx, text(object(v)["image"])); err != nil {
			t.Fatal("release deleted shared image", err)
		}
	}
}

func TestComposeRealBuildAndWritableAssets(t *testing.T) {
	if os.Getenv("ATTACKBOX_TEST_COMPOSE_ENGINE") != "1" {
		t.Skip("opt in with local Docker and offline fixture base image")
	}
	socket, image := os.Getenv("ATTACKBOX_TEST_DOCKER_SOCKET"), os.Getenv("ATTACKBOX_TEST_LAB_IMAGE")
	if socket == "" || image == "" {
		t.Fatal("explicit local socket and fixture base required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cli, err := NewCLI(CLIConfig{DockerSocket: socket, DockerConfig: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	node := "build-fixture-" + digest([]byte(time.Now().String()))[:12]
	nets := &engineTestNetworks{fakeNetworks: &fakeNetworks{nets: map[string]labnetwork.Network{}, attachments: map[string]labnetwork.Attachment{}}, cli: cli, node: node}
	m, err := New(ctx, Config{NodeID: node, StateDir: filepath.Join(t.TempDir(), "runtime"), Engine: cli, Networks: nets})
	if err != nil {
		t.Fatal(err)
	}
	requests := []Request{}
	t.Cleanup(func() {
		defer m.Close()
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, r := range requests {
			if _, err := m.Stop(cleanup, r.ID); err != nil {
				t.Error("build fixture cleanup pending", err)
			}
		}
	})
	dir, _ := filepath.Abs("../testdata/compose-build")
	d, err := m.Prepare(ctx, Bundle{Directory: dir, File: "compose.yml", Variables: map[string]string{"BASE_IMAGE": image}, EntryService: "web", EntryPort: 8080})
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		r := req(d, "build_"+owner+"_"+node, owner)
		requests = append(requests, r)
		if _, err := m.Start(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	container := func(r Request) string {
		t.Helper()
		e, err := m.get(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		items, err := cli.inventory(ctx, e.snapshot())
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range items {
			if c.Config.Labels["com.docker.compose.service"] == "web" {
				return c.ID
			}
		}
		t.Fatal("web missing")
		return ""
	}
	a, b := container(requests[0]), container(requests[1])
	env, err := cli.run(ctx, "container", "inspect", "--format", "{{json .Config.Env}}", a)
	if err != nil || !strings.Contains(string(env), "FIXTURE_LITERAL=literal$value") {
		t.Fatal("literal build argument changed", err)
	}
	read := func(id, path string) string {
		t.Helper()
		out, err := cli.run(ctx, "exec", id, "/fixture", "read", path)
		if err != nil {
			t.Fatal("read failed", path, err)
		}
		return string(out)
	}
	if read(a, "/app.txt") != "built-locally" || read(a, "/config/site.conf") != "read-only-config" || read(a, "/data/initialized") != "initialized" || read(a, "/readonly-data/initialized") != "initialized" {
		t.Fatal("build or init assets missing")
	}
	if _, err := cli.run(ctx, "exec", a, "/fixture", "write", "/data/seed.txt", "alice-only"); err != nil {
		t.Fatal("non-root target cannot write its copy", err)
	}
	if _, err := cli.run(ctx, "exec", a, "/fixture", "write", "/config/site.conf", "bad"); err == nil {
		t.Fatal("read-only directory became writable")
	}
	if read(b, "/data/seed.txt") != "original-data" {
		t.Fatal("users share writable package directory")
	}
	if _, err := m.Start(ctx, requests[0]); err != nil {
		t.Fatal(err)
	}
	if read(a, "/data/seed.txt") != "alice-only" {
		t.Fatal("repeat start reset files")
	}
	if _, err := m.Stop(ctx, requests[0].ID); err != nil {
		t.Fatal(err)
	}
	if read(b, "/data/seed.txt") != "original-data" {
		t.Fatal("release affected other user")
	}
	if _, err := os.Stat(filepath.Join(m.cfg.StateDir, "instances", requests[0].ID, "assets")); !os.IsNotExist(err) {
		t.Fatal("released copy still exists")
	}
	if err := cli.ImagePresent(ctx, text(object(object(d.Definition.Document["services"])["web"])["image"])); err != nil {
		t.Fatal("release removed built image", err)
	}
}

func (n *engineTestNetworks) ForgetNetwork(_ context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if net, ok := n.nets[id]; ok && !net.Cleaned {
		return fmt.Errorf("cleanup pending")
	}
	delete(n.nets, id)
	return nil
}
