package interactive

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SafeConn provides thread-safe writes over a gorilla/websocket connection.
type SafeConn struct {
	*websocket.Conn
	mu sync.Mutex
}

// NewSafeConn wraps a websocket.Conn for safe concurrent writes.
func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{Conn: conn}
}

// WriteMessage locks before sending data frames.
func (c *SafeConn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

// WriteControl locks before sending control frames (Ping, Pong, Close).
func (c *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteControl(messageType, data, deadline)
}

// SessionManager manages active CLI and GUI WebSocket connections, enforcing single-active preemption.
type SessionManager struct {
	mu        sync.Mutex
	activeCLI map[string]*SafeConn // key: "sandboxID:winID"
	activeGUI map[string]*SafeConn // key: "sandboxID"
}

// NewSessionManager creates a session tracking coordinator.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		activeCLI: make(map[string]*SafeConn),
		activeGUI: make(map[string]*SafeConn),
	}
}

// RegisterCLI enforces single-active terminal session preemption:
// if a connection exists for the same sandbox:winID, closes it with 4001 Preempted.
func (m *SessionManager) RegisterCLI(key string, conn *SafeConn) {
	m.mu.Lock()
	if old, exists := m.activeCLI[key]; exists && old != nil {
		_ = old.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "preempted"),
			time.Now().Add(time.Second),
		)
		_ = old.Close()
	}
	m.activeCLI[key] = conn
	m.mu.Unlock()
}

// UnregisterCLI cleans up the session entry when a connection terminates.
func (m *SessionManager) UnregisterCLI(key string, conn *SafeConn) {
	m.mu.Lock()
	if m.activeCLI[key] == conn {
		delete(m.activeCLI, key)
	}
	m.mu.Unlock()
}
