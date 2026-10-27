//go:build linux

package labcompose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"attackbox/node/labnetwork"
)

type Config struct {
	NodeID, StateDir          string
	Budget                    Budget
	MaxInstances, Concurrency int
	Engine                    Engine
	Networks                  Networks
	// Production supplies the sandbox owner's lifecycle lock here. The memory
	// registry remains useful for standalone fixtures without a sandbox runtime.
	SandboxAccess func(context.Context, string, func(context.Context, labnetwork.Sandbox) error) error
}
type entry struct {
	mu, op         sync.Mutex
	record         Record
	cancel         context.CancelFunc
	writeErr       error
	attachmentWake chan struct{} // guarded by Manager.mu
}
type Manager struct {
	mu                        sync.Mutex
	cfg                       Config
	lock                      *os.File
	entries                   map[string]*entry
	sandboxes                 map[string]labnetwork.Sandbox
	starts, cleanups, prepare chan struct{}
	closed                    bool
	active                    sync.WaitGroup
	clock                     func() time.Time
	lifetime                  context.Context
	cancelLifetime            context.CancelFunc
}

func New(ctx context.Context, cfg Config) (*Manager, error) {
	if !idPattern.MatchString(cfg.NodeID) || cfg.Engine == nil || cfg.Networks == nil {
		return nil, fmt.Errorf("node, engine and network manager required")
	}
	if cfg.MaxInstances == 0 {
		cfg.MaxInstances = 16
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 4
	}
	if cfg.MaxInstances < 1 || cfg.MaxInstances > 64 || cfg.Concurrency < 1 || cfg.Concurrency > 8 {
		return nil, fmt.Errorf("invalid runtime capacity")
	}
	cfg.Budget = defaults(cfg.Budget)
	lock, err := openLock(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Manager, error) { lock.Close(); return nil, err }
	for _, dir := range []string{"deployments", "instances"} {
		if err := privateDir(filepath.Join(cfg.StateDir, dir)); err != nil {
			return fail(err)
		}
	}
	entries, err := os.ReadDir(cfg.StateDir)
	if err != nil {
		return fail(err)
	}
	// The runtime lock proves previous preparation workers are no longer owned
	// by a live Manager. Only remove our transient build workspaces.
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".build-") {
			if err := os.RemoveAll(filepath.Join(cfg.StateDir, entry.Name())); err != nil {
				return fail(err)
			}
		}
	}
	if err := cfg.Engine.Check(ctx); err != nil {
		return fail(err)
	}
	m := &Manager{cfg: cfg, lock: lock, entries: map[string]*entry{}, sandboxes: map[string]labnetwork.Sandbox{}, starts: make(chan struct{}, cfg.Concurrency), cleanups: make(chan struct{}, cfg.Concurrency), prepare: make(chan struct{}, 1), clock: time.Now}
	m.lifetime, m.cancelLifetime = context.WithCancel(context.Background())
	dirs, err := os.ReadDir(filepath.Join(cfg.StateDir, "instances"))
	if err != nil {
		return fail(err)
	}
	for _, dir := range dirs {
		if !dir.IsDir() || !idPattern.MatchString(dir.Name()) {
			return fail(fmt.Errorf("unexpected instance directory"))
		}
		var r Record
		path, err := safePath(filepath.Join(cfg.StateDir, "instances", dir.Name()), "record.json")
		if err != nil {
			return fail(err)
		}
		if err := decodeFile(path, &r); err != nil {
			return fail(fmt.Errorf("invalid instance ledger"))
		}
		if r.Request.ID != dir.Name() || !idPattern.MatchString(r.Request.OwnerID) || !deploymentPattern.MatchString(r.Request.DeploymentID) || r.Request.ExpiresAt.IsZero() || r.NodeID != cfg.NodeID || r.Project != projectName(cfg.NodeID, r.Request.ID) || r.Networks == nil || r.Desired && r.Cleaned {
			return fail(fmt.Errorf("invalid instance identity"))
		}
		d, err := m.loadDeployment(r.Request.DeploymentID)
		if err != nil {
			return fail(err)
		}
		if !sameStrings(r.VolumeNames, volumeNames(r.Project, d)) {
			return fail(fmt.Errorf("invalid instance volumes"))
		}
		for key, n := range r.Networks {
			if n.ID != r.Request.ID+"/"+key || n.Spec.TargetID != r.Request.ID || n.Spec.OwnerID != r.Request.OwnerID {
				return fail(fmt.Errorf("invalid instance network ownership"))
			}
		}
		m.entries[r.Request.ID] = &entry{record: r}
	}
	return m, nil
}
func projectName(node, id string) string { return "ab_" + digest([]byte(node + "/" + id))[:24] }
func volumeNames(project string, d Deployment) []string {
	var out []string
	for _, key := range keys(object(d.Definition.Document["volumes"])) {
		out = append(out, project+"_"+key)
	}
	return out
}
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (m *Manager) enter() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("runtime closed")
	}
	m.active.Add(1)
	return nil
}
func (m *Manager) leave() { m.active.Done() }
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.cancelLifetime()
	for _, e := range m.entries {
		e.mu.Lock()
		if e.cancel != nil {
			e.cancel()
		}
		e.mu.Unlock()
	}
	m.mu.Unlock()
	m.active.Wait()
	return m.lock.Close()
}
func copyRecord(r Record) Record {
	r.Networks = cloneNetworks(r.Networks)
	r.VolumeNames = append([]string(nil), r.VolumeNames...)
	return r
}
func cloneNetworks(in map[string]labnetwork.Network) map[string]labnetwork.Network {
	out := map[string]labnetwork.Network{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (e *entry) snapshot() Record { e.mu.Lock(); defer e.mu.Unlock(); return copyRecord(e.record) }
func (m *Manager) persist(e *entry) error {
	if e.writeErr != nil {
		return e.writeErr
	}
	err := writeJSON(filepath.Join(m.cfg.StateDir, "instances", e.record.Request.ID, "record.json"), e.record)
	if err != nil {
		e.writeErr = fmt.Errorf("instance persistence failed; reopen runtime")
	}
	return e.writeErr
}
func (m *Manager) get(id string) (*entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[id]
	if e == nil {
		return nil, fmt.Errorf("unknown instance")
	}
	return e, nil
}

func (m *Manager) allocate(req Request, d Deployment) (*entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.entries[req.ID]; e != nil {
		old := e.snapshot()
		if old.Request.ID != req.ID || old.Request.OwnerID != req.OwnerID || old.Request.DeploymentID != req.DeploymentID || !old.Request.ExpiresAt.Equal(req.ExpiresAt) {
			return nil, fmt.Errorf("instance identity, deployment and deadline are immutable")
		}
		return e, nil
	}
	if !req.ExpiresAt.After(m.clock()) || req.ExpiresAt.After(m.clock().Add(24*time.Hour)) {
		return nil, fmt.Errorf("new instance lease must expire within 24 hours")
	}
	count := 0
	for _, e := range m.entries {
		if !e.snapshot().Cleaned {
			count++
		}
	}
	if count >= m.cfg.MaxInstances {
		return nil, fmt.Errorf("node instance capacity reached")
	}
	r := Record{NodeID: m.cfg.NodeID, Request: req, Project: projectName(m.cfg.NodeID, req.ID), Desired: true, Networks: map[string]labnetwork.Network{}}
	r.VolumeNames = volumeNames(r.Project, d)
	dir := filepath.Join(m.cfg.StateDir, "instances", req.ID)
	if err := privateDir(dir); err != nil {
		return nil, err
	}
	e := &entry{record: r}
	m.entries[req.ID] = e
	if err := m.persist(e); err != nil {
		return nil, err
	}
	return e, nil
}

func acquire(ctx context.Context, ch chan struct{}) error {
	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Start(ctx context.Context, req Request) (Status, error) {
	if err := m.enter(); err != nil {
		return Status{}, err
	}
	defer m.leave()
	if !idPattern.MatchString(req.ID) || !idPattern.MatchString(req.OwnerID) {
		return Status{}, fmt.Errorf("invalid instance identity")
	}
	d, err := m.loadDeployment(req.DeploymentID)
	if err != nil {
		return Status{}, err
	}
	e, err := m.allocate(req, d)
	if err != nil {
		return Status{}, err
	}
	e.op.Lock()
	defer e.op.Unlock()
	r := e.snapshot()
	e.mu.Lock()
	writeErr := e.writeErr
	e.mu.Unlock()
	if writeErr != nil {
		return Status{}, writeErr
	}
	if r.Cleaned || !r.Desired {
		return m.status(ctx, r, d)
	}
	if !r.Request.ExpiresAt.After(m.clock()) {
		return m.stopEntry(ctx, e, d)
	}
	if r.StartIssued {
		return m.finishStart(ctx, e, d)
	}
	deadline := r.Request.ExpiresAt
	if max := time.Now().Add(2 * time.Minute); deadline.After(max) {
		deadline = max
	}
	work, cancel := context.WithDeadline(ctx, deadline)
	e.mu.Lock()
	if !e.record.Desired {
		e.mu.Unlock()
		cancel()
		return m.status(ctx, e.snapshot(), d)
	}
	e.cancel = cancel
	e.mu.Unlock()
	defer func() { cancel(); e.mu.Lock(); e.cancel = nil; e.mu.Unlock() }()
	if err := acquire(work, m.starts); err != nil {
		return Status{}, err
	}
	defer func() { <-m.starts }()
	startErr := func() error {
		for _, s := range object(d.Definition.Document["services"]) {
			if err := m.cfg.Engine.ImagePresent(work, text(object(s)["image"])); err != nil {
				return fmt.Errorf("prepared image unavailable: prepare deployment again")
			}
		}
		for _, key := range d.Definition.Networks {
			n, err := m.cfg.Networks.CreateNetwork(work, labnetwork.NetworkSpec{TargetID: req.ID, OwnerID: req.OwnerID, Key: key, PrefixBits: d.Definition.PrefixBits})
			if err != nil {
				return err
			}
			e.mu.Lock()
			e.record.Networks[key] = n
			err = m.persist(e)
			e.mu.Unlock()
			if err != nil {
				return err
			}
		}
		r = e.snapshot()
		if err := m.prepareInstanceAssets(r, d); err != nil {
			return err
		}
		file, err := m.render(r, d)
		if err != nil {
			return err
		}
		if err := work.Err(); err != nil {
			return err
		}
		e.mu.Lock()
		if !e.record.Desired {
			e.mu.Unlock()
			return context.Canceled
		}
		e.record.StartIssued = true
		err = m.persist(e)
		r = copyRecord(e.record)
		e.mu.Unlock()
		if err != nil {
			return err
		}
		return m.cfg.Engine.Up(work, r, file)
	}()
	if startErr != nil {
		cleanupCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		status, cleanupErr := m.stopEntry(cleanupCtx, e, d)
		return status, errors.Join(startErr, cleanupErr)
	}
	return m.finishStart(ctx, e, d)
}

func (m *Manager) finishStart(ctx context.Context, e *entry, d Deployment) (Status, error) {
	r := e.snapshot()
	if m.cfg.SandboxAccess != nil {
		err := m.cfg.SandboxAccess(ctx, r.Request.OwnerID, func(ctx context.Context, s labnetwork.Sandbox) error { return m.connect(ctx, e.snapshot(), d, s) })
		if err != nil {
			status, _ := m.status(ctx, e.snapshot(), d)
			return status, err
		}
		return m.status(ctx, e.snapshot(), d)
	}
	m.mu.Lock()
	sandbox, available := m.sandboxes[r.Request.OwnerID]
	m.mu.Unlock()
	if available {
		if err := m.connect(ctx, e.snapshot(), d, sandbox); err != nil {
			status, _ := m.status(ctx, e.snapshot(), d)
			return status, err
		}
	}
	return m.status(ctx, e.snapshot(), d)
}

func (m *Manager) stopEntry(ctx context.Context, e *entry, d Deployment) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	e.mu.Lock()
	if e.writeErr != nil {
		err := e.writeErr
		e.mu.Unlock()
		return Status{}, err
	}
	e.record.Desired = false
	err := m.persist(e)
	e.mu.Unlock()
	if err != nil {
		return Status{}, err
	}
	r := e.snapshot()
	if r.Cleaned {
		return m.status(ctx, r, d)
	}
	if err := acquire(ctx, m.cleanups); err != nil {
		return Status{}, err
	}
	defer func() { <-m.cleanups }()
	// Disconnect before Compose removes services; never destroy the sandbox.
	nets, attachments := m.cfg.Networks.Snapshot()
	known := map[string]labnetwork.Network{}
	for _, n := range nets {
		if n.Spec.TargetID == r.Request.ID && n.Spec.OwnerID == r.Request.OwnerID {
			known[n.ID] = n
			r.Networks[n.Spec.Key] = n
		}
	}
	for _, a := range attachments {
		if _, ok := known[a.NetworkID]; ok && !a.Cleaned {
			if err := m.cfg.Networks.Detach(ctx, a.ID); err != nil {
				return pending(r), err
			}
		}
	}
	file, err := m.render(r, d)
	if err != nil {
		return pending(r), err
	}
	if err := m.cfg.Engine.Down(ctx, r, file); err != nil {
		return pending(r), err
	}
	for _, n := range r.Networks {
		if _, ok := known[n.ID]; !ok {
			return pending(r), fmt.Errorf("network ledger missing; cleanup unconfirmed")
		}
		if err := m.cfg.Networks.ReleaseNetwork(ctx, n.ID); err != nil {
			return pending(r), err
		}
	}
	if err := m.removeInstanceAssets(r); err != nil {
		return pending(r), err
	}
	e.mu.Lock()
	e.record.Cleaned = true
	err = m.persist(e)
	r = copyRecord(e.record)
	e.mu.Unlock()
	if err != nil {
		return pending(r), err
	}
	return m.status(ctx, r, d)
}
func pending(r Record) Status {
	return Status{ID: r.Request.ID, State: "cleanup_pending", ExpiresAt: r.Request.ExpiresAt, Attachment: "disconnected"}
}

func (m *Manager) Stop(ctx context.Context, id string) (Status, error) {
	if err := m.enter(); err != nil {
		return Status{}, err
	}
	defer m.leave()
	e, err := m.get(id)
	if err != nil {
		return Status{}, err
	}
	e.mu.Lock()
	e.record.Desired = false
	err = m.persist(e)
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()
	if err != nil {
		return Status{}, err
	}
	e.op.Lock()
	defer e.op.Unlock()
	r := e.snapshot()
	d, err := m.loadDeployment(r.Request.DeploymentID)
	if err != nil {
		return pending(r), err
	}
	return m.stopEntry(ctx, e, d)
}

func (m *Manager) Inspect(ctx context.Context, id string) (Status, error) {
	if err := m.enter(); err != nil {
		return Status{}, err
	}
	defer m.leave()
	e, err := m.get(id)
	if err != nil {
		return Status{}, err
	}
	r := e.snapshot()
	d, err := m.loadDeployment(r.Request.DeploymentID)
	if err != nil {
		return Status{}, err
	}
	return m.status(ctx, r, d)
}
func (m *Manager) status(ctx context.Context, r Record, d Deployment) (Status, error) {
	s := Status{ID: r.Request.ID, State: "starting", ExpiresAt: r.Request.ExpiresAt, Attachment: "not_connected", Readiness: "unverified"}
	if r.Cleaned {
		s.State = "stopped"
		s.Attachment = "disconnected"
		return s, nil
	}
	if !r.Desired {
		return pending(r), nil
	}
	services, err := m.cfg.Engine.Inspect(ctx, r)
	if err != nil {
		return s, err
	}
	s.Services = services
	sort.Slice(s.Services, func(i, j int) bool { return s.Services[i].Name < s.Services[j].Name })
	expected := object(d.Definition.Document["services"])
	seen := map[string]bool{}
	healthy := true
	running := true
	for _, svc := range services {
		if _, ok := expected[svc.Name]; !ok || seen[svc.Name] {
			return s, fmt.Errorf("unexpected project service")
		}
		seen[svc.Name] = true
		if d.Definition.OneShot[svc.Name] && svc.State == "exited" {
			continue
		}
		if svc.State != "running" {
			running = false
			if svc.State == "failed" || svc.State == "exited" {
				s.State = "failed"
			}
		}
		if svc.Health != "healthy" {
			healthy = false
		}
		if svc.Health == "unhealthy" && s.State != "failed" {
			s.State = "degraded"
		}
		if svc.Name == d.Definition.EntryService {
			if n, ok := r.Networks[d.Definition.EntryNetwork]; ok {
				ip := svc.IPs[n.Name]
				if ip != "" {
					s.EntryAddress = ip
					if d.Definition.EntryPort > 0 {
						s.EntryAddress = net.JoinHostPort(ip, fmt.Sprint(d.Definition.EntryPort))
					}
				}
			}
		}
	}
	if len(seen) != len(expected) {
		running = false
		if r.StartIssued && s.State == "starting" {
			s.State = "incomplete"
		}
	}
	if running && s.State == "starting" {
		s.State = "running"
	}
	if running && healthy {
		s.Ready = true
		s.Readiness = "healthy"
	}
	_, attachments := m.cfg.Networks.Snapshot()
	for _, a := range attachments {
		if a.Desired && !a.Cleaned {
			for _, n := range r.Networks {
				if a.NetworkID == n.ID {
					s.Attachment = "connected"
					if n.Spec.Key == d.Definition.EntryNetwork && a.Address.IsValid() {
						s.AttackerAddress = a.Address.Addr().String()
						s.Interface = a.GuestName
					}
				}
			}
		}
	}
	return s, nil
}

// Reap is called by the future Worker's existing expiry/reconcile loop. No
// extra timer or browser activity is necessary to enforce a persisted deadline.
func (m *Manager) Reap(ctx context.Context) error {
	if err := m.enter(); err != nil {
		return err
	}
	defer m.leave()
	m.mu.Lock()
	var ids []string
	for id, e := range m.entries {
		r := e.snapshot()
		if !r.Cleaned && (!r.Desired || !r.Request.ExpiresAt.After(m.clock())) {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := make(chan struct{}, m.cfg.Concurrency)
	for _, id := range ids {
		if err := acquire(ctx, workers); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			break
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			defer func() { <-workers }()
			_, err := m.Stop(ctx, id)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (m *Manager) connect(ctx context.Context, r Record, d Deployment, sandbox labnetwork.Sandbox) error {
	if !r.Desired || r.Cleaned || !r.Request.ExpiresAt.After(m.clock()) {
		return nil
	}
	n, ok := r.Networks[d.Definition.EntryNetwork]
	if !ok {
		return fmt.Errorf("entry network not yet prepared")
	}
	_, attachments := m.cfg.Networks.Snapshot()
	for _, a := range attachments {
		if a.NetworkID == n.ID && !a.Cleaned && (a.Sandbox.ID != sandbox.ID || a.Sandbox.Namespace != sandbox.Namespace) {
			if err := m.cfg.Networks.Detach(ctx, a.ID); err != nil {
				return err
			}
		}
	}
	_, err := m.cfg.Networks.Attach(ctx, n.ID, sandbox)
	return err
}

func (m *Manager) Deployment(ctx context.Context, id string) (Deployment, error) {
	if err := m.enter(); err != nil {
		return Deployment{}, err
	}
	defer m.leave()
	return m.loadDeployment(id)
}
func (m *Manager) Owns(id, owner string) bool {
	e, err := m.get(id)
	if err != nil {
		return false
	}
	return e.snapshot().Request.OwnerID == owner
}
