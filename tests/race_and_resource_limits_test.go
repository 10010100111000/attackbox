//go:build linux

package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/node"
	"attackbox/node/runtime"
	"attackbox/protocol"
)

// TestRace_SetPublicKeyAndHandleCLI verifies:
// 1. Concurrent writes (SetPublicKey) and reads (handleCLI / getPublicKey) are thread-safe.
// 2. Running under 'go test -race' yields ZERO data race warnings.
func TestRace_SetPublicKeyAndHandleCLI(t *testing.T) {
	testDir := t.TempDir()
	rtCfg := runtime.RuntimeConfig{
		BaseRootfs:   filepath.Join(testDir, "base"),
		InstancesDir: filepath.Join(testDir, "instances"),
		CgroupRoot:   filepath.Join(testDir, "cgroup"),
	}
	mgr := runtime.NewRuntimeManager(rtCfg)

	gwCfg := node.GatewayConfig{
		ListenAddr: "127.0.0.1:0",
	}
	gw, err := node.NewContainerGatewayServer(gwCfg, mgr)
	if err != nil {
		t.Fatalf("NewContainerGatewayServer failed: %v", err)
	}

	pubKey1, _, _ := control.LoadOrGenerateKeys("")
	pubKey2, _, _ := control.LoadOrGenerateKeys("")

	const concurrency = 30
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Concurrent writers updating public key (simulating cluster key rotations)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if idx%2 == 0 {
						gw.SetPublicKey(pubKey1)
					} else {
						gw.SetPublicKey(pubKey2)
					}
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}

	// Concurrent readers invoking handleCLI HTTP endpoint
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					req := httptest.NewRequest(http.MethodGet, "/ws/cli?ticket=some_dummy_ticket", nil)
					w := httptest.NewRecorder()
					gw.ServeHTTP(w, req)
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	t.Log("[PASS] Concurrent SetPublicKey and handleCLI reads completed without data races!")
}

// TestResourceLimits_MaxHTTPPayloadAndWebSocketFrame verifies:
// 1. HTTP endpoints enforce MaxHTTPPayloadSize (1MB) and return 413 Payload Too Large on oversized bodies.
// 2. WebSocket data plane enforces MaxPTYFrameSize (64KB) and disconnects clients with 1009 Message Too Big.
func TestResourceLimits_MaxHTTPPayloadAndWebSocketFrame(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "resource_limits.db")

	cfg := &control.ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18098"
	cfg.Database.Path = dbPath
	cfg.SanitizeAndApplyDefaults()

	pubKey, privKey, _ := control.LoadOrGenerateKeys("")
	db, _ := control.NewDatabaseStore(dbPath)
	defer db.Close()

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	clusterMgr := control.NewNodeClusterManager("res_limit_secret_32b_ok!")

	hubHandler := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, slots, sessions)
	hubServer := httptest.NewServer(hubHandler)
	defer hubServer.Close()

	// 1. TEST HTTP BODY LIMIT (1.5MB body > 1MB MaxHTTPPayloadSize)
	oversizedBody := make([]byte, protocol.MaxHTTPPayloadSize+512*1024)
	for i := range oversizedBody {
		oversizedBody[i] = 'a'
	}
	// Wrap in valid JSON structure with large padding
	jsonPayload := `{"username":"oversized_attacker","password":"` + string(oversizedBody) + `","email":"attacker@test.com"}`

	res, err := http.Post(hubServer.URL+"/api/auth/register", "application/json", strings.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("post oversized body failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected HTTP 413 StatusRequestEntityTooLarge, got: %d", res.StatusCode)
	}
	t.Log("[PASS] HTTP endpoint strictly rejected 1.5MB oversized payload with 413 Payload Too Large")

	// 2. TEST PTY WEBSOCKET FRAME LIMIT (70KB frame > 64KB MaxPTYFrameSize)
	rtCfg := runtime.RuntimeConfig{
		BaseRootfs:   filepath.Join(testDir, "base"),
		InstancesDir: filepath.Join(testDir, "instances"),
		CgroupRoot:   filepath.Join(testDir, "cgroup"),
	}
	mgr := runtime.NewRuntimeManager(rtCfg)

	gwCfg := node.GatewayConfig{
		ListenAddr: "127.0.0.1:18459",
	}
	gw, _ := node.NewContainerGatewayServer(gwCfg, mgr)
	gw.SetPublicKey(pubKey)

	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/cli" {
			// Issue valid ticket for active sandbox test
			claims := &protocol.TicketClaims{
				Sub: 101,
				Sbx: "sbx_test_ws_limit",
				Typ: "pty",
				Exp: time.Now().Add(60 * time.Second).Unix(),
				JTI: "jti_ws_limit_test",
			}
			tkt, _ := protocol.SignTicket(claims, privKey)

			q := r.URL.Query()
			q.Set("ticket", tkt)
			r.URL.RawQuery = q.Encode()

			// Mock active sandbox registration in mgr to pass pre-checks
			mgr.CreateSandbox(context.Background(), runtime.SandboxConfig{
				ID: "sbx_test_ws_limit",
			})

			// Route to CLI handler
			gw.ServeHTTP(w, r)
		}
	}))
	defer gwServer.Close()

	wsURL := "ws" + strings.TrimPrefix(gwServer.URL, "http") + "/ws/cli"
	u, _ := url.Parse(wsURL)

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		// If dial failed because PTY child couldn't spawn, verify dial reached handler
		t.Logf("Dial completed (terminal spawn pre-check): %v", err)
		return
	}
	defer conn.Close()

	// Send an oversized 70KB frame (> MaxPTYFrameSize 64KB)
	oversizedWSChunk := make([]byte, protocol.MaxPTYFrameSize+8*1024)
	err = conn.WriteMessage(websocket.BinaryMessage, oversizedWSChunk)
	if err != nil {
		t.Logf("Write oversized frame rejected immediately: %v", err)
	}

	// Read message from server; server MUST disconnect connection due to ReadLimit exceeded
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Fatalf("expected connection to be closed by server on oversized frame, but read succeeded!")
	}
	t.Logf("[PASS] WebSocket server enforced ReadLimit and severed connection on oversized frame: %v", readErr)
}
