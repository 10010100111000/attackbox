package node

import (
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SafeConn serializes data writes and keeps the underlying connection private so
// callers cannot accidentally bypass writeMu through promoted writer methods.
// The caller must still use a single ReadMessage loop.
type SafeConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// NewSafeConn takes ownership of conn; configure its read limit before wrapping.
func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{conn: conn}
}

func (c *SafeConn) ReadMessage() (int, []byte, error) {
	if c == nil || c.conn == nil {
		return 0, nil, net.ErrClosed
	}
	return c.conn.ReadMessage()
}

func (c *SafeConn) WriteMessage(messageType int, data []byte) error {
	if c == nil || c.conn == nil {
		return net.ErrClosed
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	return c.conn.WriteMessage(messageType, data)
}

// WriteControl is concurrency-safe in Gorilla and enforces its own deadline.
// Taking writeMu here would let a blocked data write defeat that deadline.
func (c *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	if c == nil || c.conn == nil {
		return net.ErrClosed
	}
	return c.conn.WriteControl(messageType, data, deadline)
}

// Close is concurrency-safe in Gorilla. It must remain independent of writeMu
// so it can interrupt a blocked reader or writer during shutdown or takeover.
func (c *SafeConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
