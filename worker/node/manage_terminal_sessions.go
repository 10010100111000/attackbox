package node

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type GatewaySessionManager struct {
	mu          sync.Mutex
	cliSessions map[string]*SafeConn
	guiSessions map[string]*SafeConn
}

func NewGatewaySessionManager() *GatewaySessionManager {
	return &GatewaySessionManager{
		cliSessions: make(map[string]*SafeConn),
		guiSessions: make(map[string]*SafeConn),
	}
}

func (sm *GatewaySessionManager) RegisterCLI(key string, conn *SafeConn) {
	var oldConn *SafeConn
	sm.mu.Lock()
	if old, exists := sm.cliSessions[key]; exists {
		oldConn = old
	}
	sm.cliSessions[key] = conn
	sm.mu.Unlock()

	// Perform preemptive closure outside global mutex lock to prevent slow WebSocket network stalls
	if oldConn != nil && oldConn != conn {
		go func() {
			_ = oldConn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(4001, "preempted by new connection"),
				time.Now().Add(time.Second),
			)
			_ = oldConn.Close()
		}()
	}
}

func (sm *GatewaySessionManager) UnregisterCLI(key string, conn *SafeConn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cur, exists := sm.cliSessions[key]; exists && cur == conn {
		delete(sm.cliSessions, key)
	}
}

func (sm *GatewaySessionManager) RegisterGUI(key string, conn *SafeConn) {
	var oldConn *SafeConn
	sm.mu.Lock()
	if old, exists := sm.guiSessions[key]; exists {
		oldConn = old
	}
	sm.guiSessions[key] = conn
	sm.mu.Unlock()

	if oldConn != nil && oldConn != conn {
		go func() {
			_ = oldConn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(4001, "preempted by new connection"),
				time.Now().Add(time.Second),
			)
			_ = oldConn.Close()
		}()
	}
}

func (sm *GatewaySessionManager) UnregisterGUI(key string, conn *SafeConn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cur, exists := sm.guiSessions[key]; exists && cur == conn {
		delete(sm.guiSessions, key)
	}
}
