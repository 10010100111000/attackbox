//go:build linux

package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/protocol"
)

// TestP1_9_CreateBoxTimeoutAndLateResponseRollback verifies:
// 1. Control Hub cancels pending RPC when rpcTimeoutSec expires.
// 2. Control Hub returns ErrRPCTimeout.
// 3. When a late CreateBoxResp arrives from a slow worker, Control Hub detects it and issues a reverse DestroyBox.
func TestP1_9_CreateBoxTimeoutAndLateResponseRollback(t *testing.T) {
	clusterSecret := "test_secret_for_p1_9_verification!"
	mgr := control.NewNodeClusterManager(clusterSecret)
	mgr.SetTimeouts(5, 1) // 1 second RPC timeout

	// Channel to catch the reverse DestroyBox issued by Control Hub
	destroyReceived := make(chan string, 1)

	// Connect mock worker to cluster manager HTTP handler
	mgrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agent/connect" {
			mgr.HandleAgentConnect(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer mgrSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(mgrSrv.URL, "http") + "/api/agent/connect?node_id=mock-worker-1"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+clusterSecret)
	headers.Set("X-Node-ID", "mock-worker-1")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("failed to connect mock worker: %v", err)
	}
	defer conn.Close()

	// Goroutine to read messages sent by Control Hub to Worker
	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := protocol.UnpackMessage(raw, &msg); err != nil {
				continue
			}
			if msg.Type == protocol.MsgTypeDestroyBox {
				var req protocol.DestroyBoxReq
				if err := msg.UnpackPayload(&req); err == nil {
					destroyReceived <- req.SandboxID
				}
			}
		}
	}()

	// 1. Send Register
	regReq := protocol.RegisterReq{
		NodeID:   "mock-worker-1",
		Version:  "2.0",
		DataPort: 8443,
	}
	regMsg, _ := protocol.NewMessage(protocol.MsgTypeRegister, regReq)
	data, _ := protocol.PackMessage(regMsg)
	_ = conn.WriteMessage(websocket.TextMessage, data)

	// Wait for registration
	time.Sleep(150 * time.Millisecond)

	// 2. Dispatch CreateBox from Control Hub with 1s timeout
	createReq := protocol.CreateBoxReq{
		SandboxID: "sbx-timeout-001",
		UserID:    101,
		SlotIndex: 1,
	}

	start := time.Now()
	_, err = mgr.CreateBox(context.Background(), "mock-worker-1", createReq)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected ErrRPCTimeout, got nil")
	}
	if err != control.ErrRPCTimeout {
		t.Fatalf("expected ErrRPCTimeout, got: %v", err)
	}
	t.Logf("[PASS] CreateBox correctly timed out after %v", elapsed)

	// 3. Worker now belatedly finishes creation (after timeout) and sends CreateBoxResp
	lateResp := protocol.CreateBoxResp{
		SandboxID: "sbx-timeout-001",
		Success:   true,
		SandboxIP: "10.200.0.11",
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeCreateBoxResp, lateResp)
	respData, _ := protocol.PackMessage(respMsg)
	_ = conn.WriteMessage(websocket.TextMessage, respData)

	// 4. Verify Control Hub immediately issues a reverse DestroyBox
	select {
	case destroyedID := <-destroyReceived:
		if destroyedID != "sbx-timeout-001" {
			t.Fatalf("expected destroyed sandbox sbx-timeout-001, got: %s", destroyedID)
		}
		t.Logf("[PASS] Control Hub successfully caught late CreateBoxResp and issued reverse DestroyBox for %s!", destroyedID)
	case <-time.After(3 * time.Second):
		t.Fatalf("P1-9 FAILED: Control Hub dropped late CreateBoxResp without issuing reverse DestroyBox!")
	}
}
