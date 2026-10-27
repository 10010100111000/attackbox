package runtime

import (
	"attackbox/node/labnetwork"
	"context"
)

// Hooks are registered before accepting controller work. The sandbox opMu is
// held while they run; callbacks must not wait on long Compose operations.
func (m *RuntimeManager) SetLabLifecycle(ready func(context.Context, labnetwork.Sandbox) error, gone func(context.Context, string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labReady = ready
	m.labGone = gone
}
func (m *RuntimeManager) notifyLabReady(ctx context.Context, s *SandboxRecord) error {
	m.mu.RLock()
	hook := m.labReady
	m.mu.RUnlock()
	if hook == nil || s.OwnerID == "" {
		return nil
	}
	return hook(ctx, labnetwork.Sandbox{ID: s.ID, OwnerID: s.OwnerID, PID: s.PID, Namespace: s.Namespace})
}
func (m *RuntimeManager) notifyLabGone(ctx context.Context, id string) error {
	m.mu.RLock()
	hook := m.labGone
	m.mu.RUnlock()
	if hook == nil {
		return nil
	}
	return hook(ctx, id)
}
func (m *RuntimeManager) WithLabSandbox(ctx context.Context, owner string, fn func(context.Context, labnetwork.Sandbox) error) error {
	for _, s := range m.ListSnapshots() {
		if s.OwnerID != owner {
			continue
		}
		m.mu.RLock()
		rec := m.sandboxes[s.ID]
		m.mu.RUnlock()
		if rec == nil {
			continue
		}
		rec.opMu.Lock()
		m.mu.RLock()
		valid := m.sandboxes[s.ID] == rec && rec.State == "running"
		snapshot := snapshotOf(rec)
		m.mu.RUnlock()
		if valid {
			err := fn(ctx, labnetwork.Sandbox{ID: snapshot.ID, OwnerID: snapshot.OwnerID, PID: snapshot.PID, Namespace: snapshot.Namespace})
			rec.opMu.Unlock()
			return err
		}
		rec.opMu.Unlock()
	}
	return nil
}
