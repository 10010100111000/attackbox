//go:build linux

package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/node"
	"attackbox/node/runtime"
	"attackbox/protocol"
)

// TestP1_10_FIFOInstructionOrdering verifies:
// When Control Hub dispatches CreateBox followed immediately by DestroyBox (e.g. user rapid stop or timeout compensation),
// Worker processes them in strict FIFO sequence per sandbox without reordering, ensuring zero sandbox or slot leaks.
func TestP1_10_FIFOInstructionOrdering(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("FIFO ordering test requires root privileges for btrfs and namespaces")
	}

	testDir := t.TempDir()
	instancesDir := filepath.Join(testDir, "instances")
	_ = os.MkdirAll(instancesDir, 0755)

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	// Channels to collect worker responses on mock control hub
	createRespCh := make(chan protocol.CreateBoxResp, 1)
	destroyRespCh := make(chan protocol.DestroyBoxResp, 1)

	// Mock Control Hub WebSocket Server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := protocol.UnpackMessage(raw, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case protocol.MsgTypeRegister:
				// Respond with RegisterAck
				ack, _ := protocol.NewMessage(protocol.MsgTypeRegisterAck, protocol.RegisterResp{
					Success: true,
					NodeID:  "node-fifo-test",
				})
				data, _ := protocol.PackMessage(ack)
				_ = conn.WriteMessage(websocket.TextMessage, data)

				// Rapidly fire CreateBox followed IMMEDIATELY by DestroyBox back-to-back
				go func() {
					time.Sleep(50 * time.Millisecond)

					targetSbxID := "sbx-fifo-order-001"

					// 1. Send CreateBox
					createReq := protocol.CreateBoxReq{
						SandboxID: targetSbxID,
						UserID:    999,
						SlotIndex: 1,
					}
					createMsg, _ := protocol.NewMessage(protocol.MsgTypeCreateBox, createReq)
					createData, _ := protocol.PackMessage(createMsg)
					_ = conn.WriteMessage(websocket.TextMessage, createData)

					// 2. Immediately send DestroyBox without any delay (back-to-back TCP frames)
					destroyReq := protocol.DestroyBoxReq{
						SandboxID: targetSbxID,
						Reason:    "user_rapid_stop",
					}
					destroyMsg, _ := protocol.NewMessage(protocol.MsgTypeDestroyBox, destroyReq)
					destroyData, _ := protocol.PackMessage(destroyMsg)
					_ = conn.WriteMessage(websocket.TextMessage, destroyData)
				}()

			case protocol.MsgTypeCreateBoxResp:
				var resp protocol.CreateBoxResp
				_ = msg.UnpackPayload(&resp)
				createRespCh <- resp

			case protocol.MsgTypeDestroyBoxResp:
				var resp protocol.DestroyBoxResp
				_ = msg.UnpackPayload(&resp)
				destroyRespCh <- resp
			}
		}
	}))
	defer srv.Close()

	// 2. Initialize Worker Node
	nodeCfg := &node.NodeConfig{
		Node: node.NodeInfoConfig{
			NodeID:               "node-fifo-test",
			ControlURL:           "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/connect",
			ClusterSecret:        "test-secret",
			HeartbeatIntervalSec: 5,
		},
		Gateway: node.GatewayConfig{
			ListenAddr: "127.0.0.1:18449",
			TLSEnabled: false,
		},
		Runtime: runtime.RuntimeConfig{
			BaseRootfs:   "/var/lib/attackbox/base_rootfs",
			InstancesDir: instancesDir,
			CgroupRoot:   "/sys/fs/cgroup/attackbox",
		},
	}

	rtMgr := runtime.NewRuntimeManager(nodeCfg.Runtime)
	if err := rtMgr.Start(context.Background()); err != nil {
		t.Fatalf("start runtime manager: %v", err)
	}
	defer rtMgr.Stop()

	client := node.NewNodeControlClient(nodeCfg, rtMgr, nil)
	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()

	client.Start(clientCtx)
	defer client.Stop()

	// 3. Wait for both responses
	var gotCreateResp bool
	var gotDestroyResp bool

	timeout := time.After(10 * time.Second)
	for !gotCreateResp || !gotDestroyResp {
		select {
		case cResp := <-createRespCh:
			gotCreateResp = true
			t.Logf("[PASS] Received CreateBoxResp: Success=%v, Error=%s", cResp.Success, cResp.Error)
		case dResp := <-destroyRespCh:
			gotDestroyResp = true
			t.Logf("[PASS] Received DestroyBoxResp: Success=%v, Error=%s", dResp.Success, dResp.Error)
			if !dResp.Success {
				t.Fatalf("expected DestroyBox to succeed, got error: %s", dResp.Error)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for CreateBoxResp and DestroyBoxResp")
		}
	}

	// 4. Verification: The sandbox MUST NOT be leaked on Worker!
	activeBoxes := rtMgr.ListSandboxes()
	for _, b := range activeBoxes {
		if b.ID == "sbx-fifo-order-001" {
			t.Fatalf("P1-10 RACE DETECTED: Sandbox sbx-fifo-order-001 leaked on Worker after rapid DestroyBox!")
		}
	}
	t.Logf("[PASS] Zero sandboxes leaked! P1-10 FIFO order strictly maintained.")
}
