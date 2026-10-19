//go:build linux

package dataserver_test

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/internal/worker/config"
	"attackbox/internal/worker/dataserver"
	"attackbox/internal/worker/reexec"
	"attackbox/internal/worker/runtime"
	"attackbox/pkg/protocol"
)

func TestMain(m *testing.M) {
	// If this test binary is re-executed as container init, dispatch and exit immediately
	if reexec.Dispatch() {
		return
	}
	os.Exit(m.Run())
}

func TestDataServerPTYWithTicket(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test: requires root privileges for container operations")
	}

	// 1. Generate Ed25519 test keypair
	pub, priv, err := ed25519.GenerateKey(nil)

	// 2. Initialize runtime and create test sandbox
	rtCfg := config.DefaultConfig().Runtime
	rtCfg.BaseRootfs = "/var/lib/attackbox/base_rootfs"
	rtCfg.InstancesDir = "/var/lib/attackbox/instances"
	rtCfg.CgroupRoot = "/sys/fs/cgroup/attackbox"

	rtMgr := runtime.NewRuntimeManager(rtCfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rtMgr.Start(ctx); err != nil {
		t.Fatalf("start runtime manager: %v", err)
	}
	defer rtMgr.Stop()

	testID := "sbx_dataserver_test_01"
	_ = rtMgr.DestroySandbox(ctx, testID)

	t.Logf("Creating sandbox %s...", testID)
	sbx, err := rtMgr.CreateSandbox(ctx, runtime.SandboxConfig{
		ID:           testID,
		Limits:       runtime.LimitsFromConfig(rtCfg.Limits),
		TmpfsTmpSize: "64M",
		TmpfsRunSize: "32M",
	})
	if err != nil {
		t.Fatalf("CreateSandbox failed: %v", err)
	}
	defer func() {
		_ = rtMgr.DestroySandbox(context.Background(), testID)
	}()
	t.Logf("Sandbox %s running with PID %d", testID, sbx.PID)

	// 3. Initialize DataServer on localhost test port :18443
	dataCfg := config.DataServerConfig{
		ListenAddr: "127.0.0.1:18443",
		TLSEnabled: false,
	}
	srv, err := dataserver.NewServer(dataCfg, rtMgr)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	srv.SetPublicKey(pub)

	go func() {
		_ = srv.Start()
	}()
	defer func() {
		_ = srv.Stop(context.Background())
	}()

	// Wait for server to bind
	time.Sleep(100 * time.Millisecond)

	// 4. Test Health Check
	resp, err := http.Get("http://127.0.0.1:18443/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed: %v (resp: %v)", err, resp)
	}

	// 5. Test Invalid/Tampered Ticket rejection
	t.Log("Testing connection with invalid ticket...")
	_, _, err = websocket.DefaultDialer.Dial("ws://127.0.0.1:18443/ws/cli?ticket=bad.ticket", nil)
	if err == nil {
		t.Fatalf("expected dial to fail with invalid ticket, but succeeded")
	}
	t.Log("Invalid ticket correctly rejected with 403 Forbidden!")

	// 6. Generate valid Ticket signed by private key
	claims := &protocol.TicketClaims{
		JTI: "jti_test_12345",
		Sub: 101,
		Sbx: testID,
		Typ: "pty",
		Exp: time.Now().Add(60 * time.Second).Unix(),
	}
	validTicket, err := protocol.SignTicket(claims, priv)
	if err != nil {
		t.Fatalf("SignTicket failed: %v", err)
	}

	// 7. Connect to PTY WebSocket using valid ticket
	t.Log("Connecting to /ws/cli with valid Ed25519 ticket...")
	u := url.URL{
		Scheme:   "ws",
		Host:     "127.0.0.1:18443",
		Path:     "/ws/cli",
		RawQuery: "ticket=" + validTicket,
	}
	wsConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed with valid ticket: %v", err)
	}
	defer wsConn.Close()
	t.Log("WebSocket connected successfully!")

	// 8. Send command into PTY
	command := "echo ATTACKBOX_PTY_OK\n"
	if err := wsConn.WriteMessage(websocket.BinaryMessage, []byte(command)); err != nil {
		t.Fatalf("write to websocket failed: %v", err)
	}

	// 9. Read output from PTY until "ATTACKBOX_PTY_OK" is received
	found := false
	deadline := time.Now().Add(5 * time.Second)
	var outputAccum strings.Builder

	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputAccum.Write(msg)
		if strings.Contains(outputAccum.String(), "ATTACKBOX_PTY_OK") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected ATTACKBOX_PTY_OK in terminal output, got: %q", outputAccum.String())
	}
	t.Logf("Terminal output received successfully: %s", strings.TrimSpace(outputAccum.String()))
}
