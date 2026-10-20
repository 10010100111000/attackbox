package node

import (
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type SafeConn struct {
	*websocket.Conn
	writeMu sync.Mutex
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{Conn: conn}
}

func (c *SafeConn) Close() (err error) {
	if c == nil || c.Conn == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("safe_conn close panic: %v", r)
		}
	}()
	return c.Conn.Close()
}

func (c *SafeConn) WriteMessage(messageType int, data []byte) (err error) {
	if c == nil || c.Conn == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("safe_conn write panic: %v", r)
		}
	}()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.Conn.WriteMessage(messageType, data)
}

func (c *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) (err error) {
	if c == nil || c.Conn == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("safe_conn write_control panic: %v", r)
		}
	}()
	return c.Conn.WriteControl(messageType, data, deadline)
}

type GatewaySessionManager struct {
	mu          sync.Mutex
	cliSessions map[string]*SafeConn
}

func NewGatewaySessionManager() *GatewaySessionManager {
	return &GatewaySessionManager{
		cliSessions: make(map[string]*SafeConn),
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
	if oldConn != nil && oldConn.Conn != nil {
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
