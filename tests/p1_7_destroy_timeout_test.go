//go:build linux

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/protocol"
)

// TestP1_7_DestroyBoxTimeoutAndMissingWorker verifies:
// 1. DestroyBox does not suffer from integer division truncation to 0 (rpc_timeout_sec: 1 -> /2 -> 0s -> immediate timeout).
// 2. DestroyBox against a non-existent / disconnected worker returns ErrWorkerNotFound instead of silently returning nil.
func TestP1_7_DestroyBoxTimeoutAndMissingWorker(t *testing.T) {
	clusterSecret := "p1_7_destroy_cluster_secret_32_chars!"
	mgr := control.NewNodeClusterManager(clusterSecret)
	// rpc_timeout_sec: 1 exposed the 1/2=0 truncation pitfall in previous versions
	mgr.SetTimeouts(5, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agent/connect" {
			mgr.HandleAgentConnect(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// 1. SCENARIO A: Worker NOT in pool MUST return ErrWorkerNotFound, never nil!
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.DestroyBox(ctx, "non_existent_worker_node", protocol.DestroyBoxReq{
		SandboxID: "sbx_test_missing_node",
		Reason:    "missing_node_audit",
	})
	if err == nil {
		t.Fatalf("[FATAL P1-7] DestroyBox on disconnected/missing worker returned nil! Expected ErrWorkerNotFound")
	}
	if !errors.Is(err, control.ErrWorkerNotFound) {
		t.Fatalf("expected ErrWorkerNotFound, got: %v", err)
	}
	t.Log("[PASS] SCENARIO A: Missing worker node correctly returned ErrWorkerNotFound")

	// 2. SCENARIO B: rpc_timeout_sec=1 must NOT truncate to 0s and immediately time out!
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/agent/connect?node_id=worker-p1-7"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+clusterSecret)
	headers.Set("X-Node-ID", "worker-p1-7")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("dial worker failed: %v", err)
	}
	defer conn.Close()

	// Worker handles DestroyBox with a fast 50ms realistic turnaround
	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			if msg.Type == protocol.MsgTypeDestroyBox {
				time.Sleep(50 * time.Millisecond) // Fast realistic destroy handling
				var req protocol.DestroyBoxReq
				_ = json.Unmarshal(msg.Payload, &req)

				respPayload, _ := json.Marshal(protocol.DestroyBoxResp{
					SandboxID: req.SandboxID,
					Success:   true,
				})
				_ = conn.WriteJSON(protocol.Message{
					ID:      msg.ID,
					Type:    protocol.MsgTypeDestroyBoxResp,
					Payload: respPayload,
				})
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Dispatch DestroyBox on reconnected worker with 1s timeout setting
	destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer destroyCancel()

	err = mgr.DestroyBox(destroyCtx, "worker-p1-7", protocol.DestroyBoxReq{
		SandboxID: "sbx_p1_7_zero_timeout_check",
		Reason:    "zero_timeout_test",
	})
	if err != nil {
		t.Fatalf("[FATAL P1-7] DestroyBox failed with error (likely 0s timeout pitfall): %v", err)
	}
	t.Log("[PASS] SCENARIO B: DestroyBox with rpc_timeout_sec=1 completed successfully without zero-value truncation timeout!")
}
