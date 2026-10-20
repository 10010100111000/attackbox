package control

import (
	"attackbox/protocol"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type StartSandboxReq struct{}

type StartSandboxResp struct {
	SandboxID string                 `json:"sandbox_id"`
	WorkerID  string                 `json:"worker_id"`
	WorkerURL string                 `json:"worker_url"`
	Ticket    string                 `json:"ticket"`
	ExpiresIn int                    `json:"expires_in"`
	SandboxIP string                 `json:"sandbox_ip"`
	SlotIndex int                    `json:"slot_index"`
	ExpiresAt time.Time              `json:"expires_at"`
	Policy    protocol.SandboxPolicy `json:"policy"`
}

type StopSandboxReq struct {
	SandboxID string `json:"sandbox_id"`
}

type StopSandboxResp struct {
	Success         bool   `json:"success"`
	SandboxID       string `json:"sandbox_id"`
	DurationSeconds int64  `json:"duration_seconds"`
}

func (s *ControlHTTPServer) handleStartSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req StartSandboxReq
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			var limit *http.MaxBytesError
			if errors.As(err, &limit) {
				writeError(w, http.StatusRequestEntityTooLarge, "request too large")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
	}
	session, created, err := s.startSandbox(r.Context(), uid)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrSandboxBusy):
			status = http.StatusConflict
		case errors.Is(err, ErrUserNotFound):
			status = http.StatusNotFound
		case errors.Is(err, ErrNoWorkersAvailable), errors.Is(err, ErrNoAvailableSlots):
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, err.Error())
		return
	}
	ticket, err := s.issueTicket(uid, session.SandboxID)
	if err != nil {
		if created {
			s.sessions.SetState(session.SandboxID, SessionStateCleanupFailed)
			s.asyncCompensatingDestroy(session.WorkerNodeID, session.SandboxID, "ticket_failed")
		}
		writeError(w, http.StatusInternalServerError, "failed to issue session ticket")
		return
	}
	endpoint, exists := s.clusterMgr.GetWorkerDataURL(session.WorkerNodeID, r.Host)
	if !exists {
		writeError(w, http.StatusServiceUnavailable, "worker node currently unreachable")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		if s.db != nil {
			_ = s.db.AddAuditLog(&uid, "sandbox_start", r.RemoteAddr, fmt.Sprintf("box=%s, worker=%s", session.SandboxID, session.WorkerNodeID))
		}
	}
	writeJSON(w, status, StartSandboxResp{SandboxID: session.SandboxID, WorkerID: session.WorkerNodeID, WorkerURL: endpoint, Ticket: ticket, ExpiresIn: s.cfg.Auth.TicketTTLSec, SandboxIP: session.SandboxIP, SlotIndex: session.SlotIndex, ExpiresAt: session.ExpiresAt, Policy: session.Policy})
}

func (s *ControlHTTPServer) handleStopSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, _, role, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req StopSandboxReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			writeError(w, http.StatusRequestEntityTooLarge, "request too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	sandboxID := req.SandboxID
	if sandboxID == "" {
		// If sandbox_id omitted, default to user's currently active sandbox
		if active, running := s.sessions.GetByUser(uid); running {
			sandboxID = active.SandboxID
		}
	}

	if sandboxID == "" {
		writeError(w, http.StatusBadRequest, "invalid request body: sandbox_id is required or no active sandbox found")
		return
	}

	session, exists := s.sessions.Get(sandboxID)
	if !exists {
		writeError(w, http.StatusNotFound, "sandbox session not found or already stopped")
		return
	}

	if session.UserID != uid && role != "admin" {
		writeError(w, http.StatusForbidden, "you do not have permission to stop this sandbox")
		return
	}

	stoppedSession, err := s.TerminateSandbox(r.Context(), sandboxID, "user_stop")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to terminate sandbox: "+err.Error())
		return
	}

	durationSec := int64(time.Since(stoppedSession.StartedAt).Seconds())
	writeJSON(w, http.StatusOK, StopSandboxResp{
		Success:         true,
		SandboxID:       stoppedSession.SandboxID,
		DurationSeconds: durationSec,
	})
}

func (s *ControlHTTPServer) handleActiveSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	session, running := s.sessions.GetByUser(uid)
	if !running {
		writeJSON(w, http.StatusOK, map[string]interface{}{"active": false})
		return
	}

	if session.State != SessionStateRunning {
		writeJSON(w, http.StatusOK, map[string]interface{}{"active": false, "sandbox_id": session.SandboxID, "state": session.State})
		return
	}
	// Strictly read-only: Do NOT touch activity timestamp on polling

	ticket, _ := s.issueTicket(uid, session.SandboxID)
	endpoint, exists := s.clusterMgr.GetWorkerDataURL(session.WorkerNodeID, r.Host)
	if !exists {
		writeError(w, http.StatusServiceUnavailable, "worker node currently unreachable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":     true,
		"sandbox_id": session.SandboxID,
		"worker_id":  session.WorkerNodeID,
		"worker_url": endpoint,
		"ticket":     ticket,
		"expires_in": s.cfg.Auth.TicketTTLSec,
		"sandbox_ip": session.SandboxIP,
		"state":      session.State,
		"started_at": session.StartedAt,
		"expires_at": session.ExpiresAt,
		"policy":     session.Policy,
	})
}
