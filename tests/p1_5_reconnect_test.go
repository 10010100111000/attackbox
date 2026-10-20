//go:build linux

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/protocol"
)

// TestP1_5_WorkerReconnectDoesNotUnregisterNewConnection specifically verifies:
// When a worker node reconnects, a new WebSocket connection is established before the old
// connection's read loop completely exits. When the old connection terminates and its defer
// unregister executes, it MUST NOT unregister or delete the newly active connection from the cluster!
func TestP1_5_WorkerReconnectDoesNotUnregisterNewConnection(t *testing.T) {
	clusterSecret := "p1_5_reconnect_cluster_secret_32_chars!"
	mgr := control.NewNodeClusterManager(clusterSecret)
	mgr.SetTimeouts(5, 5)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agent/connect" {
			mgr.HandleAgentConnect(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/agent/connect?node_id=worker-p1-5"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+clusterSecret)
	headers.Set("X-Node-ID", "worker-p1-5")

	dialer := websocket.Dialer{}

	// 1. Establish initial connection (conn1)
	conn1, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("conn1 dial failed: %v", err)
	}

	// Read loop for conn1 to process incoming messages
	conn1Done := make(chan struct{})
	go func() {
		defer close(conn1Done)
		for {
			_, _, err := conn1.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Wait for registration
	time.Sleep(100 * time.Millisecond)

	// Verify worker registered
	w, err := mgr.SelectBestWorker()
	if err != nil || w.NodeID != "worker-p1-5" {
		t.Fatalf("expected worker-p1-5 registered, got err=%v", err)
	}

	// 2. Worker reconnects: Establish replacement connection (conn2) with SAME NodeID
	conn2, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("conn2 dial failed: %v", err)
	}
	defer conn2.Close()

	rpcReceivedOnConn2 := make(chan string, 1)

	// conn2 read loop handles CreateBox RPC
	go func() {
		for {
			_, raw, err := conn2.ReadMessage()
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
				rpcReceivedOnConn2 <- req.SandboxID

				respPayload, _ := json.Marshal(protocol.CreateBoxResp{
					SandboxID: req.SandboxID,
					Success:   true,
					SandboxIP: "10.200.0.99",
				})
				_ = conn2.WriteJSON(protocol.Message{
					ID:      msg.ID,
					Type:    protocol.MsgTypeCreateBoxResp,
					Payload: respPayload,
				})
			}
		}
	}()

	// Give cluster manager time to register conn2 and automatically close conn1
	time.Sleep(100 * time.Millisecond)

	// 3. Close conn1 explicitly and wait for its goroutine to finish its deferred unregisterWorker
	_ = conn1.Close()
	select {
	case <-conn1Done:
		t.Log("[PASS] conn1 completely terminated and its defer unregister executed")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for conn1 read loop to exit")
	}

	// Additional sleep to ensure any deferred cleanup has executed
	time.Sleep(100 * time.Millisecond)

	// 4. CRITICAL ASSERTION: The cluster MUST still retain worker-p1-5 in its active pool!
	activeWorker, err := mgr.SelectBestWorker()
	if err != nil {
		t.Fatalf("[FATAL REGRESSION] worker-p1-5 was erroneously unregistered by stale conn1 exit! err: %v", err)
	}
	if activeWorker.NodeID != "worker-p1-5" {
		t.Fatalf("unexpected active worker: %s", activeWorker.NodeID)
	}

	// 5. Verify RPC functionality: Dispatched CreateBox MUST be routed to conn2 and succeed!
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	createReq := protocol.CreateBoxReq{
		SandboxID: "sbx_p1_5_reconnect_test",
		UserID:    101,
		SlotIndex: 5,
	}
	resp, err := mgr.CreateBox(ctx, "worker-p1-5", createReq)
	if err != nil {
		t.Fatalf("CreateBox on reconnected worker failed: %v", err)
	}
	if !resp.Success || resp.SandboxIP != "10.200.0.99" {
		t.Fatalf("unexpected CreateBoxResp: %+v", resp)
	}

	select {
	case sID := <-rpcReceivedOnConn2:
		if sID != "sbx_p1_5_reconnect_test" {
			t.Errorf("expected sbx_p1_5_reconnect_test on conn2, got %s", sID)
		}
		t.Log("[PASS] CreateBox RPC successfully delivered to and answered by conn2!")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for conn2 to process CreateBox RPC")
	}
}
