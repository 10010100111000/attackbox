//go:build linux

package labcompose

import (
	"context"
	"fmt"
	"log"

	"attackbox/node/labnetwork"
)

// Lifecycle callbacks run under the sandbox lock. They only record the new
// identity and signal work; they never acquire an instance's long-running op
// lock. All actual attachment work follows op -> sandbox, just like Start.
func (m *Manager) SandboxAvailable(_ context.Context, sandbox labnetwork.Sandbox) error {
	if !idPattern.MatchString(sandbox.ID) || !idPattern.MatchString(sandbox.OwnerID) || sandbox.PID <= 0 || sandbox.Namespace.Device == 0 || sandbox.Namespace.Inode == 0 {
		return fmt.Errorf("valid captured sandbox identity required")
	}
	if err := m.enter(); err != nil {
		return err
	}
	defer m.leave()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sandboxes[sandbox.OwnerID] = sandbox
	m.scheduleAttachments(sandbox.OwnerID)
	return nil
}

func (m *Manager) SandboxGone(_ context.Context, id string) error {
	if err := m.enter(); err != nil {
		return err
	}
	defer m.leave()
	m.mu.Lock()
	defer m.mu.Unlock()
	for owner, sandbox := range m.sandboxes {
		if sandbox.ID == id {
			delete(m.sandboxes, owner)
			m.scheduleAttachments(owner)
		}
	}
	return nil
}

// Called under m.mu. A bounded, coalescing signal per instance preserves events
// arriving during Up/Down or another attachment, without polling or timers.
func (m *Manager) scheduleAttachments(owner string) {
	if m.closed {
		return
	}
	for _, e := range m.entries {
		r := e.snapshot()
		if r.Request.OwnerID != owner || r.Cleaned {
			continue
		}
		if e.attachmentWake == nil {
			e.attachmentWake = make(chan struct{}, 1)
			m.active.Add(1)
			go m.attachmentLoop(e, e.attachmentWake)
		}
		select {
		case e.attachmentWake <- struct{}{}:
		default:
		}
	}
}

func (m *Manager) attachmentLoop(e *entry, wake <-chan struct{}) {
	defer m.active.Done()
	for {
		select {
		case <-m.lifetime.Done():
			return
		case <-wake:
		}
		e.op.Lock()
		if m.lifetime.Err() != nil {
			e.op.Unlock()
			return
		}
		err := m.reconnect(m.lifetime, e)
		e.op.Unlock()
		if err != nil && m.lifetime.Err() == nil {
			log.Printf("[COMPOSE] sandbox attachment for %s: %v", e.snapshot().Request.ID, err)
		}
		// Retire idle work, including stopped-instance tombstones. Producers use
		// this same lock, so a new event either stays queued or starts a new task.
		m.mu.Lock()
		if len(wake) == 0 {
			e.attachmentWake = nil
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
	}
}

func (m *Manager) reconnect(ctx context.Context, e *entry) error {
	r := e.snapshot()
	if !r.Desired || r.Cleaned || !r.StartIssued || !r.Request.ExpiresAt.After(m.clock()) {
		return nil
	}
	d, err := m.entryDeployment(e, r.Request.DeploymentID)
	if err != nil {
		return err
	}
	available := false
	connect := func(ctx context.Context, sandbox labnetwork.Sandbox) error {
		available = true
		return m.connect(ctx, e.snapshot(), d, sandbox)
	}
	if m.cfg.SandboxAccess != nil {
		// Re-read the authoritative identity under its lifecycle lock. Never reuse
		// an event's old PID after waiting for a slow Compose operation.
		if err := m.cfg.SandboxAccess(ctx, r.Request.OwnerID, connect); err != nil {
			return err
		}
	} else {
		m.mu.Lock()
		sandbox, ok := m.sandboxes[r.Request.OwnerID]
		m.mu.Unlock()
		if ok {
			if err := connect(ctx, sandbox); err != nil {
				return err
			}
		}
	}
	if available {
		return nil
	}
	_, attachments := m.cfg.Networks.Snapshot()
	for _, a := range attachments {
		if a.Cleaned || a.Sandbox.OwnerID != r.Request.OwnerID {
			continue
		}
		for _, network := range r.Networks {
			if a.NetworkID == network.ID {
				if err := m.cfg.Networks.Detach(ctx, a.ID); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}
