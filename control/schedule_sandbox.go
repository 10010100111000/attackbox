package control

import (
	"attackbox/protocol"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

var ErrSandboxBusy = errors.New("sandbox creation or cleanup is already in progress")

// startSandbox owns user reservation, scheduling and creation acknowledgement.
func (s *ControlHTTPServer) startSandbox(ctx context.Context, uid int64) (ActiveSession, bool, error) {
	if existing, ok := s.sessions.GetByUser(uid); ok && existing.State == SessionStateRunning {
		return existing, false, nil
	}
	release, ok := s.sessions.TryReserve(uid)
	if !ok {
		return ActiveSession{}, false, ErrSandboxBusy
	}
	defer release()
	if _, err := s.db.GetUserByID(uid); err != nil {
		return ActiveSession{}, false, err
	}
	worker, err := s.clusterMgr.SelectBestWorker()
	if err != nil {
		return ActiveSession{}, false, err
	}
	id := generateSandboxID()
	slot, err := s.slots.Allocate(id)
	if err != nil {
		return ActiveSession{}, false, err
	}
	now := time.Now()
	policy := s.resolvePolicy()
	session := &ActiveSession{SandboxID: id, UserID: uid, WorkerNodeID: worker.NodeID, WorkerBootID: worker.Generation(), SlotIndex: slot, State: SessionStateCreating, StartedAt: now, LastActiveAt: now, Policy: policy}
	if err := s.sessions.Add(session); err != nil {
		s.slots.Release(id)
		return ActiveSession{}, false, ErrSandboxBusy
	}
	resp, err := s.clusterMgr.CreateBox(ctx, worker.NodeID, protocol.CreateBoxReq{BootID: session.WorkerBootID, SandboxID: id, UserID: uid, SlotIndex: slot, Policy: policy, TTLSeconds: s.cfg.Sandbox.TTLSeconds, IdleSeconds: s.cfg.Sandbox.IdleSeconds})
	if err != nil {
		s.sessions.mu.Lock()
		if current := s.sessions.sessions[id]; current != nil {
			current.State = SessionStateCleanupFailed
			if resp != nil && resp.BootID == session.WorkerBootID {
				current.InventoryRevision = resp.InventoryRevision
			}
		}
		s.sessions.mu.Unlock()
		s.asyncCompensatingDestroy(worker.NodeID, id, "compensating_create_failure")
		return ActiveSession{}, false, fmt.Errorf("worker failed to create sandbox: %w", err)
	}
	s.sessions.mu.Lock()
	current := s.sessions.sessions[id]
	ready := current != nil && current.State == SessionStateCreating
	if ready {
		current.State = SessionStateRunning
		current.SandboxIP = resp.SandboxIP
		current.WorkerBootID = resp.BootID
		current.InventoryRevision = resp.InventoryRevision
		current.ExpiresAt = time.Unix(resp.ExpiresAt, 0)
		sessionCopy := *current
		s.sessions.mu.Unlock()
		return sessionCopy, true, nil
	}
	s.sessions.mu.Unlock()
	s.asyncCompensatingDestroy(worker.NodeID, id, "creation_canceled")
	return ActiveSession{}, false, ErrSandboxBusy
}

// resolvePolicy constructs the authoritative SandboxPolicy directly from ControlConfig.
// Sandboxes strictly obey machine/cluster administrative baselines. End-users are never permitted
// to inject, override, or tamper with host infrastructure resource policies.
func (s *ControlHTTPServer) resolvePolicy() protocol.SandboxPolicy {
	return protocol.SandboxPolicy{
		Resources: s.cfg.Sandbox.Resources,
		Storage:   s.cfg.Sandbox.Storage,
		Security: protocol.BoxSecurityPolicy{
			ReadOnlyProcSys: true,
			MaskSysrq:       true,
		},
	}
}

func generateSandboxID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sbx_%x", b)
}
