package control

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"attackbox/protocol"
	"github.com/gorilla/websocket"
)

type controlFixture struct {
	server  *ControlHTTPServer
	hub     *NodeClusterManager
	http    *httptest.Server
	private ed25519.PrivateKey
}

func newControlFixture(t *testing.T, withDB bool) *controlFixture {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	var db *DatabaseStore
	if withDB {
		var err error
		db, err = NewDatabaseStore(filepath.Join(t.TempDir(), "control.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
	}
	hub := NewNodeClusterManager("test-cluster-identity")
	hub.SetPublicKey(pub)
	h := NewControlHTTPServer(&ControlConfig{}, db, priv, pub, hub, nil, nil).(*ControlHTTPServer)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &controlFixture{server: h, hub: hub, http: ts, private: priv}
}

type testWorker struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	acks     chan struct{}
	boot     string
	create   func(protocol.CreateBoxReq) protocol.CreateBoxResp
	destroy  func(protocol.DestroyBoxReq) error
	destroys atomic.Int32
}

func (f *controlFixture) connectWorker(t *testing.T, boot string) *testWorker {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.http.URL, "http")+"/api/agent/connect", http.Header{"Authorization": []string{"Bearer test-cluster-identity"}, "X-Node-ID": []string{"worker"}})
	if err != nil {
		t.Fatal(err)
	}
	w := &testWorker{conn: c, acks: make(chan struct{}, 10), boot: boot}
	reg, _ := protocol.NewMessage(protocol.MsgTypeRegister, protocol.RegisterReq{NodeID: "worker", BootID: boot, DataPort: 8443, PublicDataURL: "wss://worker.example.test"})
	if err := w.send(reg); err != nil {
		t.Fatal(err)
	}
	var ack protocol.Message
	if err := c.ReadJSON(&ack); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			var msg protocol.Message
			if err := c.ReadJSON(&msg); err != nil {
				return
			}
			switch msg.Type {
			case protocol.MsgTypeHeartbeatAck:
				w.acks <- struct{}{}
			case protocol.MsgTypeCreateBox:
				var req protocol.CreateBoxReq
				_ = msg.UnpackPayload(&req)
				resp := protocol.CreateBoxResp{Success: true, SandboxID: req.SandboxID, BootID: boot, InventoryRevision: 2, ExpiresAt: time.Now().Add(time.Duration(req.TTLSeconds) * time.Second).Unix(), SandboxIP: "10.200.0.10"}
				w.mu.Lock()
				create := w.create
				w.mu.Unlock()
				if create != nil {
					resp = create(req)
				}
				out, _ := protocol.NewMessage(protocol.MsgTypeCreateBoxResp, resp)
				out.ID = msg.ID
				_ = w.send(out)
			case protocol.MsgTypeDestroyBox:
				w.destroys.Add(1)
				var req protocol.DestroyBoxReq
				_ = msg.UnpackPayload(&req)
				resp := protocol.DestroyBoxResp{SandboxID: req.SandboxID, BootID: boot, Success: true}
				w.mu.Lock()
				destroy := w.destroy
				w.mu.Unlock()
				if destroy != nil {
					if err := destroy(req); err != nil {
						resp.Success = false
						resp.Error = err.Error()
					}
				}
				out, _ := protocol.NewMessage(protocol.MsgTypeDestroyBoxResp, resp)
				out.ID = msg.ID
				_ = w.send(out)
			}
		}
	}()
	t.Cleanup(func() { _ = c.Close() })
	return w
}

func (w *testWorker) send(msg *protocol.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(msg)
}
func (w *testWorker) heartbeat(t *testing.T, seq, revision uint64, items ...protocol.SandboxSummary) {
	t.Helper()
	m, _ := protocol.NewMessage(protocol.MsgTypeHeartbeat, protocol.HeartbeatReq{BootID: w.boot, Seq: seq, InventoryRevision: revision, Sandboxes: items, ActiveSandboxes: len(items)})
	if err := w.send(m); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.acks:
	case <-time.After(time.Second):
		t.Fatal("heartbeat acknowledgement timeout")
	}
}
func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func TestInventoryCannotEraseUnacknowledgedCreation(t *testing.T) {
	f := newControlFixture(t, false)
	w := f.connectWorker(t, "boot")
	_ = f.server.slots.Occupy("new", 1)
	_ = f.server.sessions.Add(&ActiveSession{SandboxID: "new", UserID: 1, WorkerNodeID: "worker", WorkerBootID: "boot", State: SessionStateCreating})
	w.heartbeat(t, 1, 0)
	if _, ok := f.server.sessions.Get("new"); !ok {
		t.Fatal("creating instance erased")
	}
	f.server.sessions.mu.Lock()
	f.server.sessions.sessions["new"].State = SessionStateRunning
	f.server.sessions.sessions["new"].InventoryRevision = 3
	f.server.sessions.mu.Unlock()
	w.heartbeat(t, 2, 2)
	if _, ok := f.server.sessions.Get("new"); !ok {
		t.Fatal("pre-creation inventory erased running instance")
	}
	w.heartbeat(t, 3, 3)
	if _, ok := f.server.sessions.Get("new"); ok {
		t.Fatal("confirmed absence did not release instance")
	}
}

func TestCleanupFailedConfirmedAbsentReleasesUserAndSlot(t *testing.T) {
	f := newControlFixture(t, false)
	w := f.connectWorker(t, "boot")
	_ = f.server.slots.Occupy("old", 1)
	_ = f.server.sessions.Add(&ActiveSession{SandboxID: "old", UserID: 1, WorkerNodeID: "worker", WorkerBootID: "boot", InventoryRevision: 2, State: SessionStateCleanupFailed})
	w.heartbeat(t, 1, 3)
	if _, ok := f.server.slots.GetSlot("old"); ok {
		t.Fatal("slot leaked")
	}
	release, ok := f.server.sessions.TryReserve(1)
	if !ok {
		t.Fatal("user remains blocked")
	}
	release()
}

func TestControlRestartCleansUnknownBeforeScheduling(t *testing.T) {
	f := newControlFixture(t, false)
	w := f.connectWorker(t, "boot")
	if _, err := f.hub.SelectBestWorker(); err == nil {
		t.Fatal("node scheduled before inventory")
	}
	w.heartbeat(t, 1, 3, protocol.SandboxSummary{ID: "unknown", State: "running"})
	eventually(t, func() bool { return w.destroys.Load() == 1 })
	if _, err := f.hub.SelectBestWorker(); err == nil {
		t.Fatal("node scheduled with unknown leftovers")
	}
	w.heartbeat(t, 2, 4)
	if _, err := f.hub.SelectBestWorker(); err != nil {
		t.Fatal(err)
	}
}

func TestBootIdentitySurvivesConnectionReplacement(t *testing.T) {
	f := newControlFixture(t, false)
	w := f.connectWorker(t, "old")
	w.heartbeat(t, 1, 0)
	_ = f.server.slots.Occupy("old-box", 1)
	_ = f.server.sessions.Add(&ActiveSession{SandboxID: "old-box", UserID: 1, WorkerNodeID: "worker", WorkerBootID: "old", State: SessionStateRunning, InventoryRevision: 1})
	_ = w.conn.Close()
	w = f.connectWorker(t, "new")
	w.heartbeat(t, 1, 0)
	if _, ok := f.server.sessions.Get("old-box"); ok {
		t.Fatal("old boot session survived")
	}
}

func TestHeartbeatOrderAndDrainIntent(t *testing.T) {
	w := &WorkerNode{BootID: "boot", State: protocol.NodeStateDraining}
	if ok := w.UpdateHeartbeat(protocol.HeartbeatReq{BootID: "boot", Seq: 10, InventoryRevision: 2}); !ok {
		t.Fatal("valid heartbeat rejected")
	}
	if ok := w.UpdateHeartbeat(protocol.HeartbeatReq{BootID: "boot", Seq: 9, InventoryRevision: 2}); ok {
		t.Fatal("old sequence accepted")
	}
	if ok := w.UpdateHeartbeat(protocol.HeartbeatReq{BootID: "boot", Seq: 11, InventoryRevision: 1}); ok {
		t.Fatal("old inventory accepted")
	}
	if ok := w.UpdateHeartbeat(protocol.HeartbeatReq{BootID: "other", Seq: 12, InventoryRevision: 3}); ok {
		t.Fatal("other daemon generation accepted")
	}
	if w.GetState() != protocol.NodeStateDraining {
		t.Fatal("heartbeat cleared maintenance")
	}
}

func TestConcurrentStopUsesOneRPC(t *testing.T) {
	f := newControlFixture(t, false)
	w := f.connectWorker(t, "boot")
	entered, release := make(chan struct{}), make(chan struct{})
	w.mu.Lock()
	w.destroy = func(protocol.DestroyBoxReq) error { close(entered); <-release; return nil }
	w.mu.Unlock()
	_ = f.server.sessions.Add(&ActiveSession{SandboxID: "box", UserID: 1, WorkerNodeID: "worker", State: SessionStateRunning})
	done := make(chan error, 1)
	go func() { _, err := f.server.TerminateSandbox(context.Background(), "box", "test"); done <- err }()
	<-entered
	if _, err := f.server.TerminateSandbox(context.Background(), "box", "test"); err == nil {
		t.Fatal("duplicate stop accepted")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if w.destroys.Load() != 1 {
		t.Fatalf("%d destroy RPCs", w.destroys.Load())
	}
}

func TestCreateAndReuseKeepPublicWSSAddress(t *testing.T) {
	f := newControlFixture(t, true)
	w := f.connectWorker(t, "boot")
	w.heartbeat(t, 1, 0)
	u, err := f.server.db.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	token, _ := SignJWT(f.private, JWTClaims{Sub: u.ID, Username: u.Username, Role: u.Role, TokenVersion: u.TokenVersion, Exp: time.Now().Add(time.Hour).Unix()})
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/api/sandbox/start", bytes.NewBufferString("{}"))
		r.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		f.server.ServeHTTP(out, r)
		var resp StartSandboxResp
		if err := json.Unmarshal(out.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if out.Code != 201 && out.Code != 200 {
			t.Fatalf("%d %s", out.Code, out.Body.String())
		}
		if resp.WorkerURL != "wss://worker.example.test/ws/cli" {
			t.Fatalf("incorrect endpoint %s", resp.WorkerURL)
		}
		if resp.ExpiresAt.Before(time.Now()) {
			t.Fatal("incorrect worker expiration")
		}
	}
}

func TestFailedCompensationRetriedOnNextInventory(t *testing.T) {
	f := newControlFixture(t, true)
	w := f.connectWorker(t, "boot")
	w.mu.Lock()
	w.create = func(req protocol.CreateBoxReq) protocol.CreateBoxResp {
		return protocol.CreateBoxResp{SandboxID: req.SandboxID, BootID: "boot", InventoryRevision: 2, Error: "create failed"}
	}
	w.destroy = func(protocol.DestroyBoxReq) error {
		if w.destroys.Load() == 1 {
			return errors.New("temporary cleanup failure")
		}
		return nil
	}
	w.mu.Unlock()
	w.heartbeat(t, 1, 0)
	u, _ := f.server.db.GetUserByUsername("admin")
	token, _ := SignJWT(f.private, JWTClaims{Sub: u.ID, Username: u.Username, Role: u.Role, TokenVersion: u.TokenVersion, Exp: time.Now().Add(time.Hour).Unix()})
	r := httptest.NewRequest("POST", "/api/sandbox/start", bytes.NewBufferString("{}"))
	r.Header.Set("Authorization", "Bearer "+token)
	out := httptest.NewRecorder()
	f.server.ServeHTTP(out, r)
	if out.Code != 500 {
		t.Fatalf("expected failure, got %d", out.Code)
	}
	eventually(t, func() bool {
		f.server.cleanupMu.Lock()
		defer f.server.cleanupMu.Unlock()
		return w.destroys.Load() == 1 && len(f.server.cleaning) == 0
	})
	sess, ok := f.server.sessions.GetByUser(u.ID)
	if !ok || sess.State != SessionStateCleanupFailed {
		t.Fatal("lost creation failure record")
	}
	if _, held := f.server.slots.GetSlot(sess.SandboxID); !held {
		t.Fatal("slot released before cleanup")
	}
	if release, ok := f.server.sessions.TryReserve(u.ID); ok {
		release()
		t.Fatal("user can create while cleanup pending")
	}
	w.heartbeat(t, 2, 3, protocol.SandboxSummary{ID: sess.SandboxID, State: "cleanup_failed"})
	eventually(t, func() bool { _, exists := f.server.sessions.Get(sess.SandboxID); return !exists })
	if w.destroys.Load() != 2 {
		t.Fatalf("cleanup attempts=%d", w.destroys.Load())
	}
}

func TestConcurrentCreateKeepsUserReserved(t *testing.T) {
	f := newControlFixture(t, true)
	w := f.connectWorker(t, "boot")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	w.mu.Lock()
	w.create = func(req protocol.CreateBoxReq) protocol.CreateBoxResp {
		close(entered)
		<-release
		return protocol.CreateBoxResp{SandboxID: req.SandboxID, Success: true, BootID: "boot", InventoryRevision: 2, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}
	w.mu.Unlock()
	w.heartbeat(t, 1, 0)
	u, _ := f.server.db.GetUserByUsername("admin")
	token, _ := SignJWT(f.private, JWTClaims{Sub: u.ID, Username: u.Username, Role: u.Role, TokenVersion: u.TokenVersion, Exp: time.Now().Add(time.Hour).Unix()})
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/sandbox/start", bytes.NewBufferString("{}"))
		r.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		f.server.ServeHTTP(out, r)
		return out
	}
	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- request() }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first create was not dispatched")
	}
	if result := request(); result.Code != http.StatusConflict {
		t.Fatalf("duplicate create returned %d", result.Code)
	}
	sess, exists := f.server.sessions.GetByUser(u.ID)
	if !exists || sess.State != SessionStateCreating {
		t.Fatal("creation is not tracked")
	}
	once.Do(func() { close(release) })
	if result := <-first; result.Code != http.StatusCreated {
		t.Fatalf("first create failed: %s", result.Body.String())
	}
}
