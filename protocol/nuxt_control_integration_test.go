package protocol_test

import (
	"attackbox/protocol"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This opt-in test uses the actual Go wire structs against the built Nuxt hub.
// It never starts a RuntimeManager or creates/destroys a sandbox.
func TestNuxtNodeControlCompatibility(t *testing.T) {
	endpoint := os.Getenv("NUXT_CONTROL_TEST_URL")
	if endpoint == "" {
		t.Skip("NUXT_CONTROL_TEST_URL is not configured")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "ws" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		t.Fatal("test requires a local ws endpoint")
	}
	secret := os.Getenv("NODE_TEST_CLUSTER_SECRET")
	if secret == "" {
		t.Fatal("NODE_TEST_CLUSTER_SECRET is required")
	}
	headers := http.Header{"Authorization": {"Bearer " + secret}, "X-Node-ID": {"test-node"}}
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadLimit(protocol.MaxWebSocketMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	bootID := fmt.Sprintf("go-compat-%d", time.Now().UnixNano())
	exchange := func(kind string, payload any, responseKind string, target any) {
		t.Helper()
		msg, err := protocol.NewMessage(kind, payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(msg); err != nil {
			t.Fatal(err)
		}
		var response protocol.Message
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatal(err)
		}
		if response.Type != responseKind {
			t.Fatalf("unexpected response type %q", response.Type)
		}
		if err := json.Unmarshal(response.Payload, target); err != nil {
			t.Fatal(err)
		}
	}
	var registered protocol.RegisterResp
	exchange(protocol.MsgTypeRegister, protocol.RegisterReq{NodeID: "test-node", BootID: bootID, Version: "2.0", DataPort: 18443, PublicDataURL: "ws://127.0.0.1:18443", Capabilities: protocol.NodeCapabilities{CPUCores: 4, TotalMemMB: 8192, OSVersion: "linux/amd64"}}, protocol.MsgTypeRegisterAck, &registered)
	key, err := hex.DecodeString(registered.PublicKey)
	if err != nil || len(key) != 32 || !registered.Success || registered.NodeID != "test-node" {
		t.Fatal("invalid registration acknowledgement")
	}
	var heartbeat protocol.HeartbeatResp
	exchange(protocol.MsgTypeHeartbeat, protocol.HeartbeatReq{NodeID: "test-node", BootID: bootID, Seq: 1, InventoryRevision: 0, Timestamp: time.Now().Unix(), LoadAvg: 0.2, MemAvailableMB: 4096, DiskFreeGB: 80}, protocol.MsgTypeHeartbeatAck, &heartbeat)
	if !heartbeat.Success {
		t.Fatal("heartbeat rejected")
	}
}

// The browser integration writes a short-lived test ticket to a private temp file.
// Verify it with the exact Go verifier consumed by the production Worker.
func TestNuxtTicketCompatibility(t *testing.T) {
	path := os.Getenv("NUXT_TICKET_TEST_FILE")
	if path == "" {
		t.Skip("NUXT_TICKET_TEST_FILE is not configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Ticket       string `json:"ticket"`
		PublicKey    string `json:"publicKey"`
		SandboxID    string `json:"sandboxId"`
		WorkerUserID int64  `json:"workerUserId"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	key, err := hex.DecodeString(fixture.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := protocol.VerifyTicket(fixture.Ticket, key)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Sub != fixture.WorkerUserID || claims.Sbx != fixture.SandboxID || claims.Typ != "pty" || claims.JTI == "" {
		t.Fatal("Nuxt ticket does not preserve Worker identity or stream binding")
	}
}
