package runtime

import (
	"attackbox/protocol"
	"os/exec"
	"sync"
	"time"
)

type SandboxConfig struct {
	TTLSeconds  int
	IdleSeconds int
	ID          string
	UserID      int64
	SlotIndex   int
	IP          string
	Policy      protocol.SandboxPolicy
}

// SandboxRecord Worker 内部唯一的全量权威实例记录
type SandboxRecord struct {
	opMu         sync.Mutex
	State        string    `json:"state"`
	Revision     uint64    `json:"revision"`
	ExpiresAt    time.Time `json:"expires_at"`
	waitDone     chan struct{}
	idleLimit    time.Duration
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	SlotIndex    int       `json:"slot_index"`
	IP           string    `json:"ip"`
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	CleanupError string    `json:"cleanup_error,omitempty"`
	GUIRunning   bool      `json:"gui_running,omitempty"`

	cmd    *exec.Cmd `json:"-"`
	guiCmd *exec.Cmd
}

// SandboxSnapshot 外部组件只读值快照
type SandboxSnapshot struct {
	IdleSeconds  int64  `json:"idle_seconds"`
	State        string `json:"state"`
	Revision     uint64 `json:"revision"`
	ExpiresAt    int64  `json:"expires_at"`
	ID           string `json:"id"`
	UserID       int64  `json:"user_id"`
	SlotIndex    int    `json:"slot_index"`
	IP           string `json:"ip"`
	PID          int    `json:"pid"`
	StartedAt    int64  `json:"started_at"`
	LastActiveAt int64  `json:"last_active_at"`
	CleanupError string `json:"cleanup_error,omitempty"`
	GUIRunning   bool   `json:"gui_running,omitempty"`
}

func (m *RuntimeManager) ListSandboxIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, 0, len(m.sandboxes))
	for id := range m.sandboxes {
		res = append(res, id)
	}
	return res
}

func (m *RuntimeManager) TouchSandbox(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sandboxes[id]; ok {
		s.LastActiveAt = time.Now()
	}
}

func snapshotOf(s *SandboxRecord) SandboxSnapshot {
	return SandboxSnapshot{
		IdleSeconds:  int64(s.idleLimit / time.Second),
		ID:           s.ID,
		UserID:       s.UserID,
		SlotIndex:    s.SlotIndex,
		IP:           s.IP,
		PID:          s.PID,
		State:        s.State,
		Revision:     s.Revision,
		StartedAt:    s.StartedAt.Unix(),
		LastActiveAt: s.LastActiveAt.Unix(),
		ExpiresAt:    s.ExpiresAt.Unix(),
		CleanupError: s.CleanupError,
		GUIRunning:   s.GUIRunning,
	}
}

func (m *RuntimeManager) GetSnapshot(id string) (SandboxSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sandboxes[id]
	if !ok {
		return SandboxSnapshot{}, false
	}
	return snapshotOf(s), true
}

func (m *RuntimeManager) Inventory() (uint64, []protocol.SandboxSummary) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]protocol.SandboxSummary, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		res = append(res, protocol.SandboxSummary{
			ID:           s.ID,
			LastActiveAt: s.LastActiveAt.Unix(),
			State:        s.State,
			GUIRunning:   s.GUIRunning,
		})
	}
	return m.revision, res
}

func (m *RuntimeManager) ListSnapshots() []SandboxSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]SandboxSnapshot, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		res = append(res, snapshotOf(s))
	}
	return res
}
