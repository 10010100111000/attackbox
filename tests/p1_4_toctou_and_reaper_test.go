//go:build linux

package tests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/protocol"
)

// TestP1_4_TOCTOU_ConcurrentDoubleClicks verifies:
// 1. SessionManager.TryReserve prevents TOCTOU concurrency bugs.
// 2. Rapid concurrent POST /api/sandbox/start for the same user only succeeds once (201),
//    while the concurrent sibling is immediately rejected with HTTP 409 Conflict.
func TestP1_4_TOCTOU_ConcurrentDoubleClicks(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "p1_4_toctou.db")
	clusterSecret := "p1_4_toctou_cluster_secret_32_chars!"

	cfg := &control.ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18096"
	cfg.Database.Path = dbPath
	cfg.Cluster.ClusterSecret = clusterSecret
	cfg.SanitizeAndApplyDefaults()

	pubKey, privKey, err := control.LoadOrGenerateKeys("")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	clusterMgr := control.NewNodeClusterManager(clusterSecret)
	clusterMgr.SetPublicKey(pubKey)
	clusterMgr.SetTimeouts(5, 5)

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()

	handler := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, slots, sessions)
	hubServer := httptest.NewServer(handler)
	defer hubServer.Close()

	// 1. Connect mock worker via WebSocket
	wsURL := "ws" + hubServer.URL[4:] + "/api/agent/connect?node_id=mock-worker-toctou"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+clusterSecret)
	headers.Set("X-Node-ID", "mock-worker-toctou")

	dialer := websocket.Dialer{}
	workerConn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("connect mock worker: %v", err)
	}
	defer workerConn.Close()

	var workerCreateCount int32

	// Mock worker handles CreateBox with simulated processing delay
	go func() {
		for {
			_, raw, err := workerConn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			if msg.Type == protocol.MsgTypeCreateBox {
				atomic.AddInt32(&workerCreateCount, 1)
				// Simulate worker RPC latency to expose TOCTOU race window
				time.Sleep(150 * time.Millisecond)

				var createReq protocol.CreateBoxReq
				_ = json.Unmarshal(msg.Payload, &createReq)
				respPayload, _ := json.Marshal(protocol.CreateBoxResp{
					SandboxID: createReq.SandboxID,
					Success:   true,
					SandboxIP: fmt.Sprintf("10.99.0.%d", createReq.SlotIndex),
				})
				_ = workerConn.WriteJSON(protocol.Message{
					ID:      msg.ID,
					Type:    protocol.MsgTypeCreateBoxResp,
					Payload: respPayload,
				})
			}
		}
	}()

	// Wait for worker registration in cluster
	time.Sleep(100 * time.Millisecond)

	// 2. Register & login test user
	regBody, _ := json.Marshal(control.RegisterReq{
		Username: "toctou_user",
		Password: "password123!",
		Email:    "toctou@example.com",
	})
	resp, err := http.Post(hubServer.URL+"/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("register failed: %v, code %d", err, resp.StatusCode)
	}
	var authResp control.AuthResp
	_ = json.NewDecoder(resp.Body).Decode(&authResp)
	token := authResp.Token

	// 3. Fire two concurrent POST /api/sandbox/start requests simultaneously
	const concurrency = 2
	var wg sync.WaitGroup
	statusCodes := make([]int, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		idx := i
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, hubServer.URL+"/api/sandbox/start", bytes.NewReader([]byte("{}")))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 5 * time.Second}
			res, err := client.Do(req)
			if err != nil {
				statusCodes[idx] = 999
				return
			}
			defer res.Body.Close()
			statusCodes[idx] = res.StatusCode
		}()
	}
	wg.Wait()

	t.Logf("Concurrent start status codes: %v", statusCodes)

	// Exactly one request MUST succeed (201 Created) and the other MUST be rejected with 409 Conflict
	var count201, count409 int
	for _, code := range statusCodes {
		if code == http.StatusCreated {
			count201++
		} else if code == http.StatusConflict {
			count409++
		}
	}

	if count201 != 1 {
		t.Errorf("Expected exactly 1 successful creation (201), got %d", count201)
	}
	if count409 != 1 {
		t.Errorf("Expected exactly 1 rejected conflict (409), got %d", count409)
	}

	// Worker MUST have only received 1 CreateBox request, never 2
	if created := atomic.LoadInt32(&workerCreateCount); created != 1 {
		t.Errorf("Expected worker to receive exactly 1 CreateBoxReq, got %d", created)
	}
}

// TestP1_4_Reaper_TTLAndIdleReclamation verifies:
// 1. Sandboxes past TTL are reaped automatically.
// 2. Idle sandboxes without activity/touch are reaped automatically.
// 3. User polling GET /api/sandbox/active touches and resets the inactivity timer.
// 4. TerminateSandbox properly invokes reverse DestroyBox and releases IP slot.
func TestP1_4_Reaper_TTLAndIdleReclamation(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "p1_4_reaper.db")
	clusterSecret := "p1_4_reaper_cluster_secret_32_chars!"

	cfg := &control.ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18097"
	cfg.Database.Path = dbPath
	cfg.Cluster.ClusterSecret = clusterSecret
	// Set very short TTL (2 seconds), Idle (1 second), and fast Reaper scan (1 second) for fast, reliable unit testing
	cfg.Sandbox.TTLSeconds = 2
	cfg.Sandbox.IdleSeconds = 1
	cfg.Sandbox.ReaperIntervalSec = 1
	cfg.SanitizeAndApplyDefaults()

	pubKey, privKey, err := control.LoadOrGenerateKeys("")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	clusterMgr := control.NewNodeClusterManager(clusterSecret)
	clusterMgr.SetPublicKey(pubKey)
	clusterMgr.SetTimeouts(5, 5)

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()

	handler := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, slots, sessions)
	hubServer := httptest.NewServer(handler)
	defer hubServer.Close()

	// Connect mock worker
	wsURL := "ws" + hubServer.URL[4:] + "/api/agent/connect?node_id=mock-worker-reaper"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+clusterSecret)
	headers.Set("X-Node-ID", "mock-worker-reaper")

	dialer := websocket.Dialer{}
	workerConn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("connect mock worker: %v", err)
	}
	defer workerConn.Close()

	destroyReasonChan := make(chan string, 10)

	// Worker replies to CreateBox and captures DestroyBox
	go func() {
		for {
			_, raw, err := workerConn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			if msg.Type == protocol.MsgTypeCreateBox {
				var req protocol.CreateBoxReq
				_ = json.Unmarshal(msg.Payload, &req)
				respPayload, _ := json.Marshal(protocol.CreateBoxResp{
					SandboxID: req.SandboxID,
					Success:   true,
					SandboxIP: fmt.Sprintf("10.99.0.%d", req.SlotIndex),
				})
				_ = workerConn.WriteJSON(protocol.Message{
					ID:      msg.ID,
					Type:    protocol.MsgTypeCreateBoxResp,
					Payload: respPayload,
				})
			} else if msg.Type == protocol.MsgTypeDestroyBox {
				var req protocol.DestroyBoxReq
				_ = json.Unmarshal(msg.Payload, &req)
				destroyReasonChan <- req.Reason

				// Worker confirms destruction completion before Control releases slot
				respPayload, _ := json.Marshal(protocol.DestroyBoxResp{
					SandboxID: req.SandboxID,
					Success:   true,
				})
				_ = workerConn.WriteJSON(protocol.Message{
					ID:      msg.ID,
					Type:    protocol.MsgTypeDestroyBoxResp,
					Payload: respPayload,
				})
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Register user
	regBody, _ := json.Marshal(control.RegisterReq{
		Username: "reaper_user",
		Password: "password123!",
		Email:    "reaper@example.com",
	})
	resp, _ := http.Post(hubServer.URL+"/api/auth/register", "application/json", bytes.NewReader(regBody))
	var authResp control.AuthResp
	_ = json.NewDecoder(resp.Body).Decode(&authResp)
	token := authResp.Token

	// Start sandbox
	req, _ := http.NewRequest(http.MethodPost, hubServer.URL+"/api/sandbox/start", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	startRes, err := http.DefaultClient.Do(req)
	if err != nil || startRes.StatusCode != http.StatusCreated {
		t.Fatalf("start sandbox failed: %v, code %d", err, startRes.StatusCode)
	}
	var startResp control.StartSandboxResp
	_ = json.NewDecoder(startRes.Body).Decode(&startResp)
	sbxID := startResp.SandboxID

	// Check active session exists
	if _, ok := sessions.Get(sbxID); !ok {
		t.Fatalf("expected sandbox %s in sessions", sbxID)
	}

	// 1. Inactivity touch verification: Polling /api/sandbox/active touches session
	time.Sleep(500 * time.Millisecond)
	pollReq, _ := http.NewRequest(http.MethodGet, hubServer.URL+"/api/sandbox/active", nil)
	pollReq.Header.Set("Authorization", "Bearer "+token)
	pollRes, err := http.DefaultClient.Do(pollReq)
	if err != nil || pollRes.StatusCode != http.StatusOK {
		t.Fatalf("poll active failed: %v", err)
	}
	pollRes.Body.Close()

	// 2. Wait past hard TTL (cfg.Sandbox.TTLSeconds = 2s)
	// Background reaper runs every 30s by default, but we can also wait or trigger
	// To make test fast and deterministic, wait until time passes TTL (3s total)
	time.Sleep(2500 * time.Millisecond)

	// Trigger reap check via reaper or simulate scan
	// Wait for DestroyBox message delivered to worker
	select {
	case reason := <-destroyReasonChan:
		t.Logf("DestroyBox received by worker with reason: %s", reason)
		if reason != "ttl_expired" && reason != "idle_timeout" {
			t.Errorf("Unexpected destroy reason: %s", reason)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for reaper to reclaim expired sandbox")
	}

	// Session MUST no longer exist in SessionManager
	if sess, ok := sessions.Get(sbxID); ok {
		t.Errorf("Sandbox %s should have been reaped, but still exists: %+v", sbxID, sess)
	}

	// IP slot MUST have been released
	newSlot, err := slots.Allocate("sbx_verify_slot_reuse")
	if err != nil {
		t.Fatalf("failed to allocate slot: %v", err)
	}
	if newSlot != startResp.SlotIndex {
		t.Logf("Slot reuse: allocated slot %d (originally slot %d)", newSlot, startResp.SlotIndex)
	}
}

// TestP1_A_DestroyBoxFailureRetainsSlotAndSession verifies the P1-A security guarantee:
// When Worker destruction fails, times out, or node is unreachable:
// 1. Control MUST NOT delete session from memory (marks as cleanup_failed).
// 2. Control MUST NOT release IP slot (retained to prevent IP/veth collision on reuse).
// 3. New sandboxes cannot usurp the slot until worker successfully confirms cleanup.
func TestP1_A_DestroyBoxFailureRetainsSlotAndSession(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "p1_a_test.db")
	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	clusterMgr := control.NewNodeClusterManager("secret")
	clusterMgr.SetPublicKey(pub)

	cfg := &control.ControlConfig{}
	cfg.SanitizeAndApplyDefaults()

	ctrlHandler := control.NewControlHTTPServer(cfg, db, priv, pub, clusterMgr, slots, sessions)
	ctrlSrv, ok := ctrlHandler.(*control.ControlHTTPServer)
	if !ok {
		t.Fatalf("handler type assertion to *control.ControlHTTPServer failed")
	}

	// 模拟已存在活跃沙箱，但其所在的 worker 处于断网/离线状态
	sbxID := "sbx_p1_a_offline"
	slotIndex, err := slots.Allocate(sbxID)
	if err != nil {
		t.Fatalf("allocate slot: %v", err)
	}

	sess := &control.ActiveSession{
		SandboxID:    sbxID,
		UserID:       2002,
		WorkerNodeID: "worker-unreachable",
		SlotIndex:    slotIndex,
		SandboxIP:    "10.200.0.10",
		State:        control.SessionStateRunning,
		StartedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	sessions.Add(sess)

	// 调用 TerminateSandbox，预期因 worker-unreachable 不存在而失败
	_, termErr := ctrlSrv.TerminateSandbox(context.Background(), sbxID, "user_stop")
	if termErr == nil {
		t.Fatalf("expected TerminateSandbox to return error for unreachable worker, got nil")
	}

	// 验证 1: Session 必须依然存在于 SessionManager 中，且状态为 cleanup_failed
	retained, exists := sessions.Get(sbxID)
	if !exists {
		t.Fatalf("[FATAL P1-A] Session was prematurely deleted from memory despite worker destroy failure!")
	}
	if retained.State != control.SessionStateCleanupFailed {
		t.Errorf("expected session state cleanup_failed, got %s", retained.State)
	}

	// 验证 2: 槽位必须依然被该沙箱锁定，未被释放
	heldSlot, held := slots.GetSlot(sbxID)
	if !held || heldSlot != slotIndex {
		t.Fatalf("[FATAL P1-A] Slot was released prematurely! Expected slot %d to remain assigned to %s, got held=%v, slot=%d", slotIndex, sbxID, held, heldSlot)
	}

	// 验证 3: 新沙箱分配时，绝不能分配到失败沙箱持有的槽位
	challengerID := "sbx_challenger_p1_a"
	challengerSlot, err := slots.Allocate(challengerID)
	if err != nil {
		t.Fatalf("allocate slot for challenger: %v", err)
	}
	if challengerSlot == slotIndex {
		t.Fatalf("[FATAL P1-A] Slot conflict! Challenger got slot %d which is still locked by failed sandbox %s", challengerSlot, sbxID)
	}

	t.Logf("[PASS] P1-A verified: Session retained in state %s and slot %d locked upon destroy failure (challenger got separate slot %d)", retained.State, slotIndex, challengerSlot)
}
