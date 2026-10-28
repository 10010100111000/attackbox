package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type gatewayTestFrame struct {
	kind int
	body string
}

type gatewayTestPeer struct {
	conn   *SafeConn
	frames <-chan gatewayTestFrame
	closed <-chan error
}

func newGatewayTestPeer(t *testing.T, dialer *websocket.Dialer) gatewayTestPeer {
	t.Helper()
	frames := make(chan gatewayTestFrame, 512)
	closed := make(chan error, 1)
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		raw, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer raw.Close()
		for {
			kind, data, err := raw.ReadMessage()
			if err != nil {
				closed <- err
				return
			}
			frames <- gatewayTestFrame{kind: kind, body: string(data)}
		}
	}))
	t.Cleanup(server.Close)
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	raw, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = raw.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("gateway test peer did not exit after close")
		}
	})
	return gatewayTestPeer{conn: NewSafeConn(raw), frames: frames, closed: closed}
}

func TestSafeConnConcurrentMessagesAndControl(t *testing.T) {
	peer := newGatewayTestPeer(t, nil)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			if _, _, err := peer.conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = peer.conn.Close()
		select {
		case <-readerDone:
		case <-time.After(5 * time.Second):
			t.Error("client reader did not exit after close")
		}
	})
	const writers, messages, pingers = 8, 32, 4
	start := make(chan struct{})
	errorsCh := make(chan error, writers+pingers)
	var wg sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			<-start
			for message := 0; message < messages; message++ {
				if err := peer.conn.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("%d:%d", writer, message))); err != nil {
					errorsCh <- err
					return
				}
			}
		}(writer)
	}
	for pinger := 0; pinger < pingers; pinger++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for ping := 0; ping < 16; ping++ {
				if err := peer.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					errorsCh <- err
					return
				}
			}
		}()
	}
	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent writers did not finish")
	}
	close(errorsCh)
	for err := range errorsCh {
		t.Errorf("concurrent write failed: %v", err)
	}
	seen := make(map[string]bool)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for len(seen) < writers*messages {
		select {
		case frame := <-peer.frames:
			if frame.kind != websocket.BinaryMessage || seen[frame.body] {
				t.Fatalf("corrupted or duplicate frame: %#v", frame)
			}
			seen[frame.body] = true
		case <-timer.C:
			t.Fatalf("received %d/%d data frames", len(seen), writers*messages)
		}
	}
	for writer := 0; writer < writers; writer++ {
		for message := 0; message < messages; message++ {
			if !seen[fmt.Sprintf("%d:%d", writer, message)] {
				t.Fatalf("missing frame %d:%d", writer, message)
			}
		}
	}
}

// Stall only post-handshake writes. Closing the transport must interrupt them
// even while Gorilla's frame lock and SafeConn's data lock are both held.
type stalledGatewayConn struct {
	net.Conn
	stall     atomic.Bool
	entered   chan struct{}
	closed    chan struct{}
	enterOnce sync.Once
	closeOnce sync.Once
}

func (c *stalledGatewayConn) Write(data []byte) (int, error) {
	if c.stall.Load() {
		c.enterOnce.Do(func() { close(c.entered) })
		<-c.closed
		return 0, net.ErrClosed
	}
	return c.Conn.Write(data)
}

func (c *stalledGatewayConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestSafeConnControlDeadlineAndCloseInterruptStalledWrite(t *testing.T) {
	var transport *stalledGatewayConn
	dialer := &websocket.Dialer{NetDialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		transport = &stalledGatewayConn{Conn: conn, entered: make(chan struct{}), closed: make(chan struct{})}
		return transport, nil
	}}
	peer := newGatewayTestPeer(t, dialer)
	// Bypass the wrapper only for cleanup, so a broken Close implementation
	// cannot leave the intentionally stalled test goroutine behind.
	t.Cleanup(func() { _ = transport.Close() })
	transport.stall.Store(true)
	writeDone := make(chan error, 1)
	go func() { writeDone <- peer.conn.WriteMessage(websocket.BinaryMessage, []byte("stalled")) }()
	select {
	case <-transport.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("data write did not reach the transport")
	}
	controlDone := make(chan error, 1)
	go func() {
		controlDone <- peer.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(50*time.Millisecond))
	}()
	select {
	case err := <-controlDone:
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("expected a control-frame deadline error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("control deadline was blocked by the data writer")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- peer.conn.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close could not interrupt the blocked data writer")
	}
	select {
	case err := <-writeDone:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("expected closed transport error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("data writer did not exit after close")
	}
}

func TestSafeConnUninitializedWritesReturnClosed(t *testing.T) {
	for _, conn := range []*SafeConn{nil, NewSafeConn(nil)} {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("test")); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("uninitialized write reported success: %v", err)
		}
		if err := conn.WriteControl(websocket.PingMessage, nil, time.Now()); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("uninitialized control write reported success: %v", err)
		}
		if _, _, err := conn.ReadMessage(); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("uninitialized read reported success: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGatewaySessionTakeover(t *testing.T) {
	for _, kind := range []string{"cli", "gui"} {
		t.Run(kind, func(t *testing.T) {
			manager := NewGatewaySessionManager()
			register, unregister := manager.RegisterCLI, manager.UnregisterCLI
			if kind == "gui" {
				register, unregister = manager.RegisterGUI, manager.UnregisterGUI
			}
			old, next := newGatewayTestPeer(t, nil), newGatewayTestPeer(t, nil)
			register("session", old.conn)
			register("session", old.conn)
			if err := old.conn.WriteMessage(websocket.TextMessage, []byte("same connection")); err != nil {
				t.Fatalf("registering the same connection closed it: %v", err)
			}
			register("session", next.conn)
			select {
			case err := <-old.closed:
				var closed *websocket.CloseError
				if !errors.As(err, &closed) || closed.Code != 4001 {
					t.Fatalf("expected takeover close 4001, got %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("old session was not closed")
			}
			unregister("session", old.conn)
			manager.mu.Lock()
			current := manager.cliSessions["session"]
			if kind == "gui" {
				current = manager.guiSessions["session"]
			}
			manager.mu.Unlock()
			if current != next.conn {
				t.Fatal("old cleanup removed the replacement session")
			}
			if err := next.conn.WriteMessage(websocket.BinaryMessage, []byte("new connection")); err != nil {
				t.Fatal(err)
			}
			select {
			case frame := <-next.frames:
				if frame.body != "new connection" {
					t.Fatalf("unexpected replacement frame: %#v", frame)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("replacement session stopped delivering data")
			}
			unregister("session", next.conn)
		})
	}
}
