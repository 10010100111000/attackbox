package control

import (
	"attackbox/protocol"
	"errors"
	"sync"
	"time"
)

type SessionState string

const (
	SessionStateCreating      SessionState = "creating"
	SessionStateRunning       SessionState = "running"
	SessionStateTerminating   SessionState = "terminating"
	SessionStateCleanupFailed SessionState = "cleanup_failed"
	SessionStateTerminated    SessionState = "terminated"
)

type ActiveSession struct {
	WorkerBootID      string                 `json:"-"`
	InventoryRevision uint64                 `json:"-"`
	SandboxID         string                 `json:"sandbox_id"`
	UserID            int64                  `json:"user_id"`
	WorkerNodeID      string                 `json:"worker_node_id"`
	SlotIndex         int                    `json:"slot_index"`
	SandboxIP         string                 `json:"sandbox_ip"`
	State             SessionState           `json:"state"`
	StartedAt         time.Time              `json:"started_at"`
	LastActiveAt      time.Time              `json:"last_active_at"`
	ExpiresAt         time.Time              `json:"expires_at"`
	Policy            protocol.SandboxPolicy `json:"policy"`
	GUIRunning        bool                   `json:"gui_running,omitempty"`
}

type SessionManager struct {
	mu        sync.RWMutex
	sessions  map[string]*ActiveSession
	byUser    map[int64]string
	reserving map[int64]time.Time // in-flight sandbox creation tracking to eliminate TOCTOU race
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:  make(map[string]*ActiveSession),
		byUser:    make(map[int64]string),
		reserving: make(map[int64]time.Time),
	}
}

// TryReserve atomically checks if a user already has an active sandbox or a creation in-flight.
func (sm *SessionManager) TryReserve(userID int64) (release func(), ok bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. Check if user already has an active sandbox
	if sbxID, exists := sm.byUser[userID]; exists {
		if _, active := sm.sessions[sbxID]; active {
			return nil, false
		}
		delete(sm.byUser, userID)
	}

	// 2. Check if creation is already in-flight (TOCTOU protection against rapid double-clicks)
	if _, busy := sm.reserving[userID]; busy {
		return nil, false
	}
	reservedAt := time.Now()
	sm.reserving[userID] = reservedAt

	releaseFunc := func() {
		sm.mu.Lock()
		defer sm.mu.Unlock()
		if sm.reserving[userID] == reservedAt {
			delete(sm.reserving, userID)
		}
	}

	return releaseFunc, true
}

func (sm *SessionManager) Add(s *ActiveSession) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if id, exists := sm.byUser[s.UserID]; exists && id != s.SandboxID {
		return errors.New("user already owns a sandbox")
	}
	delete(sm.reserving, s.UserID)
	cp := *s
	sm.sessions[s.SandboxID] = &cp
	sm.byUser[s.UserID] = s.SandboxID
	return nil
}

func (sm *SessionManager) SetState(sandboxID string, state SessionState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[sandboxID]; ok {
		s.State = state
	}
}

func (sm *SessionManager) SetGUIRunning(sandboxID string, running bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[sandboxID]; ok {
		s.GUIRunning = running
	}
}

func (sm *SessionManager) Touch(sandboxID string, t time.Time) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[sandboxID]; ok {
		if t.After(s.LastActiveAt) {
			s.LastActiveAt = t
		}
	}
}

// Get returns a defensive value copy to eliminate pointer read-write data races.
func (sm *SessionManager) Get(sandboxID string) (ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[sandboxID]
	if !ok {
		return ActiveSession{}, false
	}
	return *s, true
}

// GetByUser returns a defensive value copy to eliminate pointer read-write data races.
func (sm *SessionManager) GetByUser(userID int64) (ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sbxID, ok := sm.byUser[userID]
	if !ok {
		return ActiveSession{}, false
	}
	s, ok := sm.sessions[sbxID]
	if !ok {
		return ActiveSession{}, false
	}
	return *s, true
}

func (sm *SessionManager) Remove(sandboxID string) (ActiveSession, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.sessions[sandboxID]
	if ok {
		cp := *s
		delete(sm.sessions, sandboxID)
		if sm.byUser[s.UserID] == sandboxID {
			delete(sm.byUser, s.UserID)
		}
		return cp, true
	}
	return ActiveSession{}, false
}

// ListAll returns a snapshot slice of session value copies.
func (sm *SessionManager) ListAll() []ActiveSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	res := make([]ActiveSession, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		res = append(res, *s)
	}
	return res
}
