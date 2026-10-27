//go:build linux

package labnetwork

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"sort"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.Mutex
	cfg      Config
	store    *store
	state    state
	docker   dockerBackend
	links    linkBackend
	guard    protection
	pin      func(Sandbox) (*PinnedNamespace, error)
	closed   bool
	writeErr error
}

// New opens a private, single-writer ledger. It does not change networking or
// start a goroutine. Explicit CreateNetwork/Attach opt into host protection.
func New(cfg Config) (*Manager, error) {
	d, err := newLocalDocker(cfg.DockerSocket)
	if err != nil {
		return nil, err
	}
	l, err := newLocalLinks()
	if err != nil {
		d.Close()
		return nil, err
	}
	m, err := newManager(cfg, d, l, nftProtection{node: cfg.NodeID}, PinNamespace)
	if err != nil {
		d.Close()
		l.Close()
	}
	return m, err
}

func newManager(cfg Config, d dockerBackend, l linkBackend, g protection, pin func(Sandbox) (*PinnedNamespace, error)) (*Manager, error) {
	if !identityPattern.MatchString(cfg.NodeID) {
		return nil, fmt.Errorf("invalid node identity")
	}
	if !cfg.Pool.IsValid() {
		cfg.Pool = netip.MustParsePrefix("10.201.0.0/16")
	}
	if _, err := Plan(cfg.Pool, NetworkSpec{TargetID: "validation", OwnerID: "validation", Key: "default"}, nil); err != nil {
		return nil, err
	}
	for _, p := range cfg.Excluded {
		if !p.IsValid() {
			return nil, fmt.Errorf("invalid excluded prefix")
		}
	}
	s, loaded, err := openStore(cfg.StateDir, cfg.NodeID)
	if err != nil {
		return nil, err
	}
	for _, n := range loaded.Networks {
		if !n.Cleaned && (!cfg.Pool.Contains(n.Subnet.Addr()) || cfg.Pool.Bits() > n.Subnet.Bits()) {
			s.close()
			return nil, fmt.Errorf("active network lies outside configured pool")
		}
	}
	return &Manager{cfg: cfg, store: s, state: loaded, docker: d, links: l, guard: g, pin: pin}, nil
}

func (m *Manager) usable(ctx context.Context) error {
	if m.closed {
		return fmt.Errorf("network manager is closed")
	}
	if m.writeErr != nil {
		return fmt.Errorf("network ledger write failed; reopen manager before retry: %w", m.writeErr)
	}
	return ctx.Err()
}

func (m *Manager) save() error {
	if err := m.store.save(m.state); err != nil {
		m.writeErr = err
		return fmt.Errorf("persist network intent: %w", err)
	}
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	m.docker.Close()
	m.links.Close()
	return m.store.close()
}

func (m *Manager) CreateNetwork(ctx context.Context, spec NetworkSpec) (Network, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.usable(ctx); err != nil {
		return Network{}, err
	}
	if spec.PrefixBits == 0 {
		spec.PrefixBits = 28
	}
	if err := validateSpec(spec); err != nil {
		return Network{}, err
	}
	id := networkID(spec)
	n, exists := m.state.Networks[id]
	if exists && (n.Spec != spec || !n.Desired || n.Cleaned) {
		return Network{}, fmt.Errorf("network identity is immutable or released")
	}
	if err := m.guard.Ensure(ctx); err != nil {
		return Network{}, err
	}
	if !exists {
		networks, err := m.docker.Networks(ctx)
		if err != nil {
			return Network{}, err
		}
		occupied, err := occupiedDocker(networks)
		if err != nil {
			return Network{}, err
		}
		routes, err := m.links.Occupied(ctx)
		if err != nil {
			return Network{}, err
		}
		occupied = append(occupied, routes...)
		occupied = append(occupied, m.cfg.Excluded...)
		for _, old := range m.state.Networks {
			if !old.Cleaned {
				occupied = append(occupied, old.Subnet)
			}
		}
		n, err = Plan(m.cfg.Pool, spec, occupied)
		if err != nil {
			return Network{}, err
		}
		for _, old := range m.state.Networks {
			if !old.Cleaned && (old.Bridge == n.Bridge || old.Name == n.Name) {
				return Network{}, fmt.Errorf("generated network name collision")
			}
		}
		for _, old := range networks {
			if old.Name == n.Name || old.Options["com.docker.network.bridge.name"] == n.Bridge {
				return Network{}, fmt.Errorf("Docker network name collision or missing ledger")
			}
		}
		m.state.Networks[id] = n
		if err := m.save(); err != nil {
			return Network{}, err
		}
	}
	dockerID, err := m.docker.Ensure(ctx, n, m.cfg.NodeID)
	if err != nil {
		return n, err
	}
	n.DockerID = dockerID
	m.state.Networks[id] = n
	if err := m.save(); err != nil {
		return n, err
	}
	return n, nil
}

func (m *Manager) Attach(ctx context.Context, id string, s Sandbox) (Attachment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.usable(ctx); err != nil {
		return Attachment{}, err
	}
	n, ok := m.state.Networks[id]
	if !ok || !n.Desired || n.Cleaned || n.DockerID == "" {
		return Attachment{}, fmt.Errorf("network is not ready for attachment")
	}
	if s.OwnerID != n.Spec.OwnerID {
		return Attachment{}, fmt.Errorf("sandbox and lab owner mismatch")
	}
	ns, err := m.pin(s)
	if err != nil {
		return Attachment{}, err
	}
	defer ns.Close()
	if err := m.guard.Ensure(ctx); err != nil {
		return Attachment{}, err
	}
	if err := m.docker.Verify(ctx, n, m.cfg.NodeID); err != nil {
		return Attachment{}, err
	}
	a := makeAttachment(m.cfg.NodeID, n, s, 0)
	old, exists := m.state.Attachments[a.ID]
	if exists && !old.Cleaned {
		if !old.Desired || old.Sandbox != s {
			return Attachment{}, fmt.Errorf("attachment cleanup or identity change pending")
		}
		a = old
	} else {
		used := map[string]bool{}
		for _, other := range m.state.Attachments {
			if other.Cleaned {
				continue
			}
			if other.NetworkID == id {
				return Attachment{}, fmt.Errorf("network sandbox address already reserved")
			}
			if other.HostName == a.HostName {
				return Attachment{}, fmt.Errorf("generated interface name collision")
			}
			if other.Sandbox.ID == s.ID && other.Sandbox.Namespace == s.Namespace {
				used[other.GuestName] = true
			}
		}
		found := false
		for slot := 0; slot < 256; slot++ {
			candidate := makeAttachment(m.cfg.NodeID, n, s, slot)
			if !used[candidate.GuestName] {
				a = candidate
				found = true
				break
			}
		}
		if !found {
			return Attachment{}, fmt.Errorf("sandbox lab interface slots exhausted")
		}
		m.state.Attachments[a.ID] = a
		if err := m.save(); err != nil {
			return Attachment{}, err
		}
	}
	if err := m.links.Ensure(ctx, n, a, ns); err != nil {
		// Persist a tombstone before rollback; if it cannot be persisted, leave
		// responsibility in the ledger instead of performing unrecorded deletion.
		a.Desired = false
		m.state.Attachments[a.ID] = a
		if saveErr := m.save(); saveErr != nil {
			return a, errors.Join(err, saveErr)
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a, errors.Join(err, m.cleanupAttachment(cleanupCtx, a))
	}
	return a, nil
}

func (m *Manager) cleanupAttachment(ctx context.Context, a Attachment) error {
	if a.Cleaned {
		return nil
	}
	if err := m.links.Remove(ctx, a); err != nil {
		return fmt.Errorf("attachment cleanup pending: %w", err)
	}
	a.Cleaned = true
	a.Desired = false
	m.state.Attachments[a.ID] = a
	return m.save()
}

func (m *Manager) Detach(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.usable(ctx); err != nil {
		return err
	}
	a, ok := m.state.Attachments[id]
	if !ok {
		return fmt.Errorf("unknown attachment")
	}
	if a.Cleaned {
		return nil
	}
	a.Desired = false
	m.state.Attachments[id] = a
	if err := m.save(); err != nil {
		return err
	}
	return m.cleanupAttachment(ctx, a)
}

func (m *Manager) cleanupNetwork(ctx context.Context, n Network) error {
	if n.Cleaned {
		return nil
	}
	for _, a := range m.state.Attachments {
		if a.NetworkID == n.ID && !a.Cleaned {
			a.Desired = false
			m.state.Attachments[a.ID] = a
			if err := m.save(); err != nil {
				return err
			}
			if err := m.cleanupAttachment(ctx, a); err != nil {
				return err
			}
		}
	}
	if err := m.links.CheckBridgeEmpty(ctx, n); err != nil {
		return err
	}
	if err := m.docker.Remove(ctx, n, m.cfg.NodeID); err != nil {
		return fmt.Errorf("network cleanup pending: %w", err)
	}
	n.Cleaned = true
	n.Desired = false
	m.state.Networks[n.ID] = n
	return m.save()
}

// ReleaseNetwork is called after the project's service containers are removed.
// It never stops containers, removes images, or deletes unknown bridge ports.
func (m *Manager) ReleaseNetwork(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.usable(ctx); err != nil {
		return err
	}
	n, ok := m.state.Networks[id]
	if !ok {
		return fmt.Errorf("unknown network")
	}
	if n.Cleaned {
		return nil
	}
	n.Desired = false
	m.state.Networks[id] = n
	if err := m.save(); err != nil {
		return err
	}
	return m.cleanupNetwork(ctx, n)
}

// Reconcile is explicitly called at startup or by the future runtime's existing
// reconciliation loop; this package creates no timers. Unknown resources are
// reported, never swept. A stale namespace is disconnected, never reselected.
func (m *Manager) Reconcile(ctx context.Context) (ReconcileReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var report ReconcileReport
	if err := m.usable(ctx); err != nil {
		return report, err
	}
	for _, n := range m.state.Networks {
		if !n.Cleaned {
			if err := m.guard.Ensure(ctx); err != nil {
				return report, err
			}
			break
		}
	}
	var errs []error
	for _, n := range m.state.Networks {
		if n.Cleaned {
			continue
		}
		if !n.Desired {
			if err := m.cleanupNetwork(ctx, n); err != nil {
				report.Pending = append(report.Pending, n.ID)
				errs = append(errs, err)
			}
		} else if err := m.docker.Verify(ctx, n, m.cfg.NodeID); err != nil {
			report.Pending = append(report.Pending, n.ID)
			errs = append(errs, err)
		}
		if m.writeErr != nil {
			return report, errors.Join(errs...)
		}
	}
	known := map[string]bool{}
	for _, a := range m.state.Attachments {
		if a.Cleaned {
			continue
		}
		known[a.Alias] = true
		stale := !a.Desired
		var pinned *PinnedNamespace
		if !stale {
			ns, err := m.pin(a.Sandbox)
			pinned = ns
			if err != nil {
				if errors.Is(err, ErrNamespaceChanged) || errors.Is(err, os.ErrNotExist) {
					stale = true
				} else {
					errs = append(errs, err)
					report.Pending = append(report.Pending, a.ID)
					continue
				}
			}
		}
		if stale {
			a.Desired = false
			m.state.Attachments[a.ID] = a
			if err := m.save(); err != nil {
				return report, err
			}
			if err := m.cleanupAttachment(ctx, a); err != nil {
				errs = append(errs, err)
				report.Pending = append(report.Pending, a.ID)
			} else {
				report.Detached = append(report.Detached, a.ID)
			}
		} else {
			present, err := m.links.Check(ctx, m.state.Networks[a.NetworkID], a, pinned)
			pinned.Close()
			if err != nil || !present {
				report.Pending = append(report.Pending, a.ID)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
		if m.writeErr != nil {
			return report, errors.Join(errs...)
		}
	}
	orphans, err := m.links.Orphans(ctx, m.cfg.NodeID, known)
	if err != nil {
		errs = append(errs, err)
	}
	report.Orphans = append(report.Orphans, orphans...)
	networks, err := m.docker.Networks(ctx)
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, n := range networks {
			if n.Labels[labelNode] != m.cfg.NodeID {
				continue
			}
			record, ok := m.state.Networks[n.Labels[labelKey]]
			if !ok || record.Cleaned {
				report.Orphans = append(report.Orphans, n.Name)
			}
		}
	}
	sort.Strings(report.Pending)
	sort.Strings(report.Detached)
	sort.Strings(report.Orphans)
	return report, errors.Join(errs...)
}

func (m *Manager) Snapshot() ([]Network, []Attachment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var networks []Network
	var attachments []Attachment
	for _, n := range m.state.Networks {
		networks = append(networks, n)
	}
	for _, a := range m.state.Attachments {
		attachments = append(attachments, a)
	}
	sort.Slice(networks, func(i, j int) bool { return networks[i].ID < networks[j].ID })
	sort.Slice(attachments, func(i, j int) bool { return attachments[i].ID < attachments[j].ID })
	return networks, attachments
}
