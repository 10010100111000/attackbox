package labcompose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"attackbox/node/labnetwork"
)

type fakeEngine struct {
	mu                               sync.Mutex
	records                          map[string]Record
	services                         map[string][]Service
	prepareCount, upCount, downCount int
	downErr                          error
	blockID                          string
	started                          chan struct{}
}

func (e *fakeEngine) Check(context.Context) error { return nil }
func (e *fakeEngine) PrepareImage(context.Context, string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prepareCount++
	return "sha256:" + strings.Repeat("a", 64), nil
}
func (e *fakeEngine) ImagePresent(context.Context, string) error { return nil }
func (e *fakeEngine) Up(ctx context.Context, r Record, file string) error {
	e.mu.Lock()
	e.upCount++
	e.records[r.Request.ID] = r
	blocked := e.blockID == r.Request.ID
	e.mu.Unlock()
	if blocked {
		close(e.started)
		<-ctx.Done()
		return ctx.Err()
	}
	var doc map[string]any
	if err := decodeFile(file, &doc); err != nil {
		return err
	}
	var svcs []Service
	for _, name := range keys(object(doc["services"])) {
		ips := map[string]string{}
		for _, n := range r.Networks {
			ips[n.Name] = "10.201.0.2"
		}
		state := "running"
		if name == "init" {
			state = "exited"
		}
		svcs = append(svcs, Service{Name: name, State: state, Health: "healthy", IPs: ips})
	}
	e.mu.Lock()
	e.services[r.Request.ID] = svcs
	e.mu.Unlock()
	return nil
}
func (e *fakeEngine) Inspect(_ context.Context, r Record) ([]Service, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]Service(nil), e.services[r.Request.ID]...), nil
}
func (e *fakeEngine) Down(_ context.Context, r Record, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.downCount++
	if e.downErr != nil {
		return e.downErr
	}
	delete(e.services, r.Request.ID)
	delete(e.records, r.Request.ID)
	return nil
}

type fakeNetworks struct {
	mu          sync.Mutex
	nets        map[string]labnetwork.Network
	attachments map[string]labnetwork.Attachment
}

func (n *fakeNetworks) CreateNetwork(_ context.Context, s labnetwork.NetworkSpec) (labnetwork.Network, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	id := s.TargetID + "/" + s.Key
	if v, ok := n.nets[id]; ok {
		if !v.Desired {
			return v, errors.New("released network")
		}
		return v, nil
	}
	v := labnetwork.Network{Spec: s, ID: id, Name: "net_" + digest([]byte(id))[:16], Desired: true, DockerID: "dockerid"}
	n.nets[id] = v
	return v, nil
}
func (n *fakeNetworks) ReleaseNetwork(_ context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	v := n.nets[id]
	v.Desired = false
	v.Cleaned = true
	n.nets[id] = v
	return nil
}
func (n *fakeNetworks) Attach(_ context.Context, id string, s labnetwork.Sandbox) (labnetwork.Attachment, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := id + "/" + s.ID
	v := labnetwork.Attachment{ID: key, NetworkID: id, Sandbox: s, Desired: true}
	n.attachments[key] = v
	return v, nil
}
func (n *fakeNetworks) Detach(_ context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	v := n.attachments[id]
	v.Desired = false
	v.Cleaned = true
	n.attachments[id] = v
	return nil
}
func (n *fakeNetworks) Snapshot() ([]labnetwork.Network, []labnetwork.Attachment) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var nets []labnetwork.Network
	var as []labnetwork.Attachment
	for _, v := range n.nets {
		nets = append(nets, v)
	}
	for _, v := range n.attachments {
		as = append(as, v)
	}
	return nets, as
}
func fixtureManager(t *testing.T) (*Manager, *fakeEngine, *fakeNetworks, Bundle) {
	t.Helper()
	e := &fakeEngine{records: map[string]Record{}, services: map[string][]Service{}, started: make(chan struct{})}
	n := &fakeNetworks{nets: map[string]labnetwork.Network{}, attachments: map[string]labnetwork.Attachment{}}
	cfg := Config{NodeID: "node-a", StateDir: filepath.Join(t.TempDir(), "runtime"), Engine: e, Networks: n}
	m, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m, e, n, packageFile(t, twoServices)
}
func prepare(t *testing.T, m *Manager, b Bundle) Deployment {
	t.Helper()
	d, err := m.Prepare(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func req(d Deployment, id, owner string) Request {
	return Request{ID: id, OwnerID: owner, DeploymentID: d.ID, ExpiresAt: time.Now().UTC().Add(30 * time.Minute)}
}

func TestPreparedProjectLifecycleAndFrozenIdentity(t *testing.T) {
	m, e, n, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	r := req(d, "one", "alice")
	status, err := m.Start(ctx, r)
	if err != nil || status.State != "running" || !status.Ready || status.EntryAddress != "10.201.0.2:8080" {
		t.Fatal(status, err)
	}
	count := e.prepareCount
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	if e.prepareCount != count {
		t.Fatal("startup prepared images")
	}
	if e.upCount != 1 {
		t.Fatal("duplicate start reran initialization", e.upCount)
	}
	changed := r
	changed.ExpiresAt = changed.ExpiresAt.Add(time.Minute)
	if _, err := m.Start(ctx, changed); err == nil {
		t.Fatal("deadline changed")
	}
	if _, err := m.Start(ctx, req(d, "two", "bob")); err != nil {
		t.Fatal(err)
	}
	if e.records["one"].Project == e.records["two"].Project || e.records["one"].VolumeNames[0] == e.records["two"].VolumeNames[0] {
		t.Fatal("users share project data")
	}
	if status, err = m.Stop(ctx, r.ID); err != nil || status.State != "stopped" {
		t.Fatal(status, err)
	}
	if status, err = m.Start(ctx, r); err != nil || status.State != "stopped" {
		t.Fatal("tombstone revived", status, err)
	}
	nets, _ := n.Snapshot()
	for _, net := range nets {
		if net.Spec.TargetID == "two" && net.Cleaned {
			t.Fatal("other project released")
		}
	}
}

func TestBothSandboxOrdersAndRecreation(t *testing.T) {
	for _, first := range []bool{true, false} {
		t.Run(fmt.Sprint(first), func(t *testing.T) {
			m, _, _, b := fixtureManager(t)
			ctx := context.Background()
			d := prepare(t, m, b)
			r := req(d, "one", "alice")
			s := labnetwork.Sandbox{ID: "sandbox", OwnerID: "alice", PID: 123, Namespace: labnetwork.NamespaceIdentity{Device: 1, Inode: 2}}
			if first {
				if err := m.SandboxAvailable(ctx, s); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := m.Start(ctx, r); err != nil {
				t.Fatal(err)
			}
			if !first {
				if err := m.SandboxAvailable(ctx, s); err != nil {
					t.Fatal(err)
				}
			}
			status, err := m.Inspect(ctx, r.ID)
			if err != nil || status.Attachment != "connected" {
				t.Fatal(status, err)
			}
			if err := m.SandboxGone(ctx, s.ID); err != nil {
				t.Fatal(err)
			}
			status, err = m.Inspect(ctx, r.ID)
			if err != nil || status.State != "running" || status.Attachment == "connected" {
				t.Fatal(status, err)
			}
			s.ID = "new-sandbox"
			s.Namespace.Inode++
			if err := m.SandboxAvailable(ctx, s); err != nil {
				t.Fatal(err)
			}
			status, err = m.Inspect(ctx, r.ID)
			if err != nil || status.Attachment != "connected" {
				t.Fatal(status, err)
			}
		})
	}
}

func TestExpiryAndCleanupFailureSurviveRestart(t *testing.T) {
	m, e, _, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	r := req(d, "one", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	m.clock = func() time.Time { return r.ExpiresAt.Add(time.Second) }
	e.downErr = errors.New("Docker unavailable")
	if err := m.Reap(ctx); err == nil {
		t.Fatal("cleanup failure hidden")
	}
	s, err := m.Inspect(ctx, r.ID)
	if err != nil || s.State != "cleanup_pending" {
		t.Fatal(s, err)
	}
	cfg := m.cfg
	m.Close()
	m2, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	e.downErr = nil
	if err := m2.Reap(ctx); err != nil {
		t.Fatal(err)
	}
	if s, err := m2.Start(ctx, r); err != nil || s.State != "stopped" {
		t.Fatal(s, err)
	}
}

func TestStopCancelsLongStartWithoutBlockingOtherProject(t *testing.T) {
	m, e, _, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	e.blockID = "slow"
	slow := req(d, "slow", "alice")
	done := make(chan error, 1)
	go func() { _, err := m.Start(ctx, slow); done <- err }()
	select {
	case <-e.started:
	case <-time.After(3 * time.Second):
		t.Fatal("start not reached")
	}
	if _, err := m.Start(ctx, req(d, "fast", "bob")); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { _, err := m.Stop(ctx, slow.ID); stopped <- err }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop blocked behind long startup")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("startup not canceled", err)
	}
}

func TestPreparedFilesAreSnapshotsAndTamperIsRejected(t *testing.T) {
	m, _, _, b := fixtureManager(t)
	content := strings.Replace(twoServices, "    image: fixture:local", "    image: fixture:local\n    environment: {TOKEN: 'literal$$token'}", 1)
	content = strings.Replace(content, "volumes: [data:/data]", "volumes: [data:/data, './init.sql:/init.sql:ro']", 1)
	os.WriteFile(filepath.Join(b.Directory, b.File), []byte(content), 0600)
	os.WriteFile(filepath.Join(b.Directory, "init.sql"), []byte("select 1;"), 0600)
	d := prepare(t, m, b)
	os.WriteFile(filepath.Join(b.Directory, "init.sql"), []byte("select 2;"), 0600)
	if _, err := m.loadDeployment(d.ID); err != nil {
		t.Fatal("source change altered frozen deployment", err)
	}
	path := filepath.Join(m.cfg.StateDir, "deployments", d.ID, "files", "init.sql")
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), req(d, "one", "alice")); err == nil {
		t.Fatal("tampered prepared file accepted")
	}
}

func TestHealthAndPartialStartupAreNotReportedReady(t *testing.T) {
	m, e, _, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	r := req(d, "one", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	original := append([]Service(nil), e.services[r.ID]...)
	for i := range e.services[r.ID] {
		if e.services[r.ID][i].Name == "web" {
			e.services[r.ID][i].Health = "unhealthy"
		}
	}
	e.mu.Unlock()
	if s, err := m.Inspect(ctx, r.ID); err != nil || s.Ready || s.State != "degraded" {
		t.Fatal(s, err)
	}
	e.mu.Lock()
	e.services[r.ID] = original[:1]
	e.mu.Unlock()
	if s, err := m.Inspect(ctx, r.ID); err != nil || s.Ready || s.State != "incomplete" {
		t.Fatal(s, err)
	}
}

func TestRejectedStartCanStillBeReleasedWithVerifiedStopIdentity(t *testing.T) {
	m, e, _, b := fixtureManager(t)
	m.cfg.MaxInstances = 1
	ctx := context.Background()
	d := prepare(t, m, b)
	if _, err := m.Start(ctx, req(d, "one", "alice")); err != nil {
		t.Fatal(err)
	}
	r := req(d, "two", "bob")
	if _, err := m.Start(ctx, r); err == nil {
		t.Fatal("capacity ignored")
	}
	if s, err := m.StopRequest(ctx, r); err != nil || s.State != "stopped" {
		t.Fatal(s, err)
	}
	if s, err := m.Start(ctx, r); err != nil || s.State != "stopped" || e.upCount != 1 {
		t.Fatal("canceled request revived", s, err)
	}
	r.OwnerID = "alice"
	if _, err := m.StopRequest(ctx, r); err == nil {
		t.Fatal("foreign stop identity accepted")
	}
}
