//go:build linux

package labnetwork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type fakeDocker struct {
	networks                             map[string]Network
	ensureErr, removeErr, errorInventory error
	calls                                int
	extras                               []dockerNetwork
	afterEnsure                          func()
}

func (d *fakeDocker) Close() {}
func (d *fakeDocker) Networks(context.Context) ([]dockerNetwork, error) {
	var out []dockerNetwork
	for _, n := range d.networks {
		x := dockerNetwork{ID: n.DockerID, Name: n.Name, Options: networkOptions(n), Labels: networkLabels(n, "node-a")}
		x.IPAM.Config = append(x.IPAM.Config, struct {
			Subnet             string
			Gateway            string
			IPRange            string
			AuxiliaryAddresses map[string]string
		}{Subnet: n.Subnet.String()})
		out = append(out, x)
	}
	return append(out, d.extras...), d.errorInventory
}
func (d *fakeDocker) Ensure(_ context.Context, n Network, _ string) (string, error) {
	if d.ensureErr != nil {
		return "", d.ensureErr
	}
	d.calls++
	n.DockerID = "docker-" + n.Name
	d.networks[n.ID] = n
	if d.afterEnsure != nil {
		d.afterEnsure()
	}
	return n.DockerID, nil
}
func (d *fakeDocker) Verify(_ context.Context, n Network, _ string) error {
	if actual, ok := d.networks[n.ID]; !ok || actual.DockerID != n.DockerID {
		return fmt.Errorf("missing or replaced network")
	}
	return nil
}
func (d *fakeDocker) Remove(_ context.Context, n Network, _ string) error {
	if d.removeErr != nil {
		return d.removeErr
	}
	delete(d.networks, n.ID)
	return nil
}

type fakeLinks struct {
	attachments                   map[string]Attachment
	ensureErr, removeErr, busyErr error
	orphan                        []string
	routes                        []netip.Prefix
	removed                       int
	beforeRemove                  func()
}

func (l *fakeLinks) Close()                                           {}
func (l *fakeLinks) Occupied(context.Context) ([]netip.Prefix, error) { return l.routes, nil }
func (l *fakeLinks) Ensure(_ context.Context, _ Network, a Attachment, _ *PinnedNamespace) error {
	l.attachments[a.ID] = a
	return l.ensureErr
}
func (l *fakeLinks) Remove(_ context.Context, a Attachment) error {
	if l.beforeRemove != nil {
		l.beforeRemove()
	}
	if l.removeErr != nil {
		return l.removeErr
	}
	delete(l.attachments, a.ID)
	l.removed++
	return nil
}
func (l *fakeLinks) Check(_ context.Context, _ Network, a Attachment, _ *PinnedNamespace) (bool, error) {
	_, ok := l.attachments[a.ID]
	return ok, nil
}
func (l *fakeLinks) CheckBridgeEmpty(context.Context, Network) error { return l.busyErr }
func (l *fakeLinks) Orphans(context.Context, string, map[string]bool) ([]string, error) {
	return l.orphan, nil
}

type fakeGuard struct {
	err   error
	calls int
}

func (g *fakeGuard) Ensure(context.Context) error { g.calls++; return g.err }

type harness struct {
	cfg    Config
	d      *fakeDocker
	l      *fakeLinks
	g      *fakeGuard
	pinErr error
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{cfg: Config{NodeID: "node-a", StateDir: filepath.Join(t.TempDir(), "state"), Pool: netip.MustParsePrefix("10.201.0.0/16")}, d: &fakeDocker{networks: map[string]Network{}}, l: &fakeLinks{attachments: map[string]Attachment{}}, g: &fakeGuard{}}
}
func (h *harness) open(t *testing.T) *Manager {
	t.Helper()
	m, err := newManager(h.cfg, h.d, h.l, h.g, func(s Sandbox) (*PinnedNamespace, error) {
		if h.pinErr != nil {
			return nil, h.pinErr
		}
		f, err := os.Open("/dev/null")
		return &PinnedNamespace{file: f, Identity: s.Namespace}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}
func sandbox() Sandbox {
	return Sandbox{ID: "sandbox-a", OwnerID: "user-a", PID: 123, Namespace: NamespaceIdentity{Device: 4, Inode: 5}}
}

func TestLifecycleIdempotencyAndMultiLabSlots(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.Attach(ctx, n.ID, sandbox())
	if err != nil {
		t.Fatal(err)
	}
	again, err := m.Attach(ctx, n.ID, sandbox())
	if err != nil || again != first {
		t.Fatalf("duplicate: %+v %v", again, err)
	}
	n2, err := m.CreateNetwork(ctx, spec("two"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Attach(ctx, n2.ID, sandbox())
	if err != nil {
		t.Fatal(err)
	}
	if first.GuestName != "lab0" || second.GuestName != "lab1" || first.Address == second.Address {
		t.Fatal(first, second)
	}
	if err := m.ReleaseNetwork(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	if len(h.l.attachments) != 1 {
		t.Fatal("unrelated lab detached")
	}
	if _, err := m.CreateNetwork(ctx, spec("one")); err == nil {
		t.Fatal("released identity revived")
	}
	if err := m.ReleaseNetwork(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Attach(ctx, n.ID, sandbox()); err == nil {
		t.Fatal("released network attached")
	}
}

func TestNoSideEffectsOnNamespaceOwnerAndProtectionFailure(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	h.g.err = errors.New("nft unavailable")
	if _, err := m.CreateNetwork(ctx, spec("one")); err == nil || h.d.calls != 0 {
		t.Fatal("unprotected network created")
	}
	h.g.err = nil
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	s := sandbox()
	s.OwnerID = "user-b"
	if _, err := m.Attach(ctx, n.ID, s); err == nil {
		t.Fatal("foreign owner accepted")
	}
	h.pinErr = ErrNamespaceChanged
	if _, err := m.Attach(ctx, n.ID, sandbox()); !errors.Is(err, ErrNamespaceChanged) {
		t.Fatal(err)
	}
	if len(h.l.attachments) != 0 {
		t.Fatal("namespace mismatch changed networking")
	}
}

func TestRollbackPersistsCleanupIntentAndRetriesAfterRestart(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	h.l.ensureErr = errors.New("address setup failed")
	h.l.removeErr = errors.New("device busy")
	h.l.beforeRemove = func() {
		b, err := os.ReadFile(filepath.Join(h.cfg.StateDir, "state.json"))
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, a := range m.state.Attachments {
			if !a.Desired {
				found = true
			}
		}
		if !found || len(b) == 0 {
			t.Fatal("missing stop intent before removal")
		}
	}
	if _, err := m.Attach(ctx, n.ID, sandbox()); err == nil {
		t.Fatal("failure swallowed")
	}
	_, as := m.Snapshot()
	if len(as) != 1 || as[0].Desired || as[0].Cleaned {
		t.Fatal(as)
	}
	m.Close()
	h.l.beforeRemove = nil
	h.l.removeErr = nil
	h.l.ensureErr = nil
	m = h.open(t)
	r, err := m.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Detached) != 1 || len(h.l.attachments) != 0 {
		t.Fatal(r)
	}
	_, as = m.Snapshot()
	if !as[0].Cleaned {
		t.Fatal(as)
	}
}

func TestNetworkDeletionFailureRetainsReservationAndDoesNotRevive(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	h.d.removeErr = errors.New("active endpoint")
	if err := m.ReleaseNetwork(ctx, n.ID); err == nil {
		t.Fatal("cleanup failure swallowed")
	}
	next, err := m.CreateNetwork(ctx, spec("two"))
	if err != nil {
		t.Fatal(err)
	}
	if next.Subnet == n.Subnet {
		t.Fatal("cleanup-pending subnet reused")
	}
	if _, err := m.CreateNetwork(ctx, spec("one")); err == nil {
		t.Fatal("stop tombstone ignored")
	}
	h.d.removeErr = nil
	if _, err := m.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileStaleNamespaceAndReportOrphansWithoutDeleting(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Attach(ctx, n.ID, sandbox()); err != nil {
		t.Fatal(err)
	}
	h.pinErr = ErrNamespaceChanged
	h.l.orphan = []string{"vlh_unknown"}
	r, err := m.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Detached) != 1 || len(r.Orphans) != 1 || h.l.removed != 1 {
		t.Fatal(r)
	}
	if len(h.d.networks) != 1 {
		t.Fatal("stale sandbox deleted live lab")
	}
}

func TestBusyForeignPortPreventsBridgeRemoval(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	h.l.busyErr = errors.New("unowned port")
	if err := m.ReleaseNetwork(ctx, n.ID); err == nil || len(h.d.networks) != 1 {
		t.Fatal("foreign bridge port destroyed")
	}
}

func TestLedgerExclusiveLockAndTamperRejection(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	if _, err := newManager(h.cfg, h.d, h.l, h.g, m.pin); err == nil {
		t.Fatal("second state writer allowed")
	}
	if _, err := m.CreateNetwork(context.Background(), spec("one")); err != nil {
		t.Fatal(err)
	}
	m.Close()
	if err := os.WriteFile(filepath.Join(h.cfg.StateDir, "state.json"), []byte(`{"version":1,"node_id":"other"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newManager(h.cfg, h.d, h.l, h.g, PinNamespace); err == nil {
		t.Fatal("corrupt ledger accepted")
	}
}

func TestParallelAttachGetsOneReservedAddress(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Attach(ctx, n.ID, sandbox()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(h.l.attachments) != 1 {
		t.Fatal(h.l.attachments)
	}
}

func TestPinRejectsWorkerNamespaceAndMismatch(t *testing.T) {
	id, err := CaptureNamespace(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	s := sandbox()
	s.PID = os.Getpid()
	s.Namespace = id
	if ns, err := PinNamespace(s); err == nil {
		ns.Close()
		t.Fatal("host namespace accepted")
	}
	s.Namespace.Inode++
	if _, err := PinNamespace(s); !errors.Is(err, ErrNamespaceChanged) {
		t.Fatal(err)
	}
}

func TestCrashAfterDockerCreateResumesFromDurableIntent(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	h.d.afterEnsure = func() { m.store.dir = filepath.Join(h.cfg.StateDir, "missing-directory") }
	if _, err := m.CreateNetwork(ctx, spec("one")); err == nil {
		t.Fatal("failed commit hidden")
	}
	if len(h.d.networks) != 1 {
		t.Fatal("fixture did not reach Docker creation")
	}
	calls := h.d.calls
	if _, err := m.CreateNetwork(ctx, spec("two")); err == nil || h.d.calls != calls {
		t.Fatal("writes continued after ambiguous persistence failure")
	}
	m.Close()
	h.d.afterEnsure = nil
	m = h.open(t)
	n, err := m.CreateNetwork(ctx, spec("one"))
	if err != nil {
		t.Fatal(err)
	}
	if n.DockerID == "" || len(h.d.networks) != 1 {
		t.Fatal("pending intent duplicated network")
	}
}

func TestReservationSurvivesPrepareFailureAndDetectsForeignAddressPool(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	h.d.ensureErr = errors.New("Docker unavailable")
	first, err := m.CreateNetwork(ctx, spec("one"))
	if err == nil {
		t.Fatal("prepare failure hidden")
	}
	h.d.ensureErr = nil
	second, err := m.CreateNetwork(ctx, spec("two"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Subnet == second.Subnet {
		t.Fatal("unconfirmed reservation reused")
	}
	h2 := newHarness(t)
	h2.l.routes = []netip.Prefix{h2.cfg.Pool}
	m2 := h2.open(t)
	if _, err := m2.CreateNetwork(ctx, spec("blocked")); err == nil || h2.d.calls != 0 {
		t.Fatal("overlapping host route accepted")
	}
}

func TestCleanedTombstonesAreNotPersistedToDisk(t *testing.T) {
	h := newHarness(t)
	m := h.open(t)
	ctx := context.Background()
	n, err := m.CreateNetwork(ctx, spec("target-0001"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := m.Attach(ctx, n.ID, sandbox())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Detach(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.ReleaseNetwork(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(h.cfg.StateDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved state
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Networks) != 0 || len(saved.Attachments) != 0 {
		t.Fatalf("expected state.json to have 0 records, got %d nets, %d attachments", len(saved.Networks), len(saved.Attachments))
	}
}
