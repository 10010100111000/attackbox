package control

import (
	"attackbox/protocol"
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// TerminateSandbox terminates a sandbox session, rolls back cluster resources,
// releases IP slots, and records an audit log. Shared identically by user Stop and background Reaper.
// State machine: Running -> Terminating -> Terminated (Success) or CleanupFailed (Retained for retry).
func (s *ControlHTTPServer) TerminateSandbox(ctx context.Context, sandboxID string, reason string) (ActiveSession, error) {
	s.sessions.mu.Lock()
	stored, exists := s.sessions.sessions[sandboxID]
	if !exists || stored.State == SessionStateTerminating {
		s.sessions.mu.Unlock()
		return ActiveSession{}, errors.New("sandbox not found or already stopping")
	}
	session := *stored
	stored.State = SessionStateTerminating
	s.sessions.mu.Unlock()

	// 2. Dispatch reverse DestroyBox to worker node
	destroyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	err := s.clusterMgr.DestroyBox(destroyCtx, session.WorkerNodeID, protocol.DestroyBoxReq{
		SandboxID: session.SandboxID,
		Reason:    reason,
	})

	if err != nil {
		// Retain session and slot to prevent reused IP conflict or orphaned resources
		s.sessions.mu.Lock()
		if cur := s.sessions.sessions[sandboxID]; cur != nil && cur.State == SessionStateTerminating {
			cur.State = SessionStateCleanupFailed
		}
		s.sessions.mu.Unlock()
		log.Printf("[ERROR] Worker %s failed to destroy sandbox %s: %v (marked as cleanup_failed)", session.WorkerNodeID, sandboxID, err)
		return session, fmt.Errorf("worker cleanup failed: %w", err)
	}

	// 3. Worker confirmed destruction: mark Terminated, release slot, and remove from active list
	removedSession, _ := s.purgeSessionLocally(session.SandboxID)

	// 4. Record audit log
	durationSec := int64(time.Since(session.StartedAt).Seconds())
	if s.db != nil {
		_ = s.db.AddAuditLog(&session.UserID, "sandbox_terminate", "", fmt.Sprintf("box=%s, reason=%s, duration=%ds", session.SandboxID, reason, durationSec))
	}

	return removedSession, nil
}

// purgeSessionLocally releases the IP slot, marks the session as Terminated, and removes it from active tracking.
func (s *ControlHTTPServer) purgeSessionLocally(id string) (ActiveSession, bool) {
	s.sessions.mu.Lock()
	defer s.sessions.mu.Unlock()
	rec, ok := s.sessions.sessions[id]
	s.slots.Release(id)
	if !ok {
		return ActiveSession{}, false
	}
	delete(s.sessions.sessions, id)
	if s.sessions.byUser[rec.UserID] == id {
		delete(s.sessions.byUser, rec.UserID)
	}
	return *rec, true
}

// Cleanup is retried by subsequent heartbeats, with at most one RPC per instance.
func (s *ControlHTTPServer) asyncCompensatingDestroy(workerID, id, reason string) {
	s.cleanupMu.Lock()
	if s.cleaning[id] {
		s.cleanupMu.Unlock()
		return
	}
	s.cleaning[id] = true
	s.cleanupMu.Unlock()
	go func() {
		defer func() { s.cleanupMu.Lock(); delete(s.cleaning, id); s.cleanupMu.Unlock() }()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.clusterMgr.DestroyBox(ctx, workerID, protocol.DestroyBoxReq{SandboxID: id, Reason: reason}); err == nil {
			s.purgeSessionLocally(id)
		} else {
			s.sessions.SetState(id, SessionStateCleanupFailed)
			log.Printf("[CONTROL] Cleanup pending for %s: %v", id, err)
		}
	}()
}
