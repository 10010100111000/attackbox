//go:build linux

package tests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/node"
	"attackbox/node/runtime"
)

func TestDistributedE2E_ConcurrentStress(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Stress test requires root privileges")
	}

	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "stress.db")

	clusterSecret := "stress_cluster_secret_min_32_chars!"
	ctrlCfg := &control.ControlConfig{}
	ctrlCfg.Server.ListenAddr = "127.0.0.1:18081"
	ctrlCfg.Server.CORSAllowedOrigins = []string{"*"}
	ctrlCfg.Database.Path = dbPath
	ctrlCfg.Cluster.ClusterSecret = clusterSecret
	ctrlCfg.Cluster.HeartbeatTimeoutSec = 10
	ctrlCfg.Auth.JWTExpireHours = 1
	ctrlCfg.Auth.TicketTTLSec = 60

	pubKey, privKey, _ := control.LoadOrGenerateKeys("")
	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	clusterMgr := control.NewNodeClusterManager(clusterSecret)
	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()

	router := control.NewControlHTTPServer(ctrlCfg, db, privKey, pubKey, clusterMgr, slots, sessions)

	ctrlSrv := &http.Server{
		Addr:    ctrlCfg.Server.ListenAddr,
		Handler: router,
	}
	go func() { _ = ctrlSrv.ListenAndServe() }()
	defer func() { _ = ctrlSrv.Shutdown(context.Background()) }()

	nodeCfg := &node.NodeConfig{
		Node: node.NodeInfoConfig{
			NodeID:               "node-stress-test",
			ControlURL:           "ws://127.0.0.1:18081/api/agent/connect",
			ClusterSecret:        clusterSecret,
			HeartbeatIntervalSec: 3,
		},
		Gateway: node.GatewayConfig{
			ListenAddr: "127.0.0.1:18444",
			TLSEnabled: false,
		},
		Runtime: runtime.RuntimeConfig{
			BaseRootfs:   "/var/lib/attackbox/base_rootfs",
			InstancesDir: "/var/lib/attackbox/instances",
			CgroupRoot:   "/sys/fs/cgroup/attackbox",
		},
	}

	_ = node.EnsureHostNetwork(nodeCfg.Network)

	rtMgr := runtime.NewRuntimeManager(nodeCfg.Runtime)
	if err := rtMgr.Start(context.Background()); err != nil {
		t.Fatalf("start rtMgr: %v", err)
	}

	gwSrv, _ := node.NewContainerGatewayServer(nodeCfg.Gateway, rtMgr)
	gwSrv.SetPublicKey(pubKey)
	go func() { _ = gwSrv.Start() }()
	defer func() { _ = gwSrv.Stop(context.Background()) }()

	client := node.NewNodeControlClient(nodeCfg, rtMgr, func(pk ed25519.PublicKey) {
		gwSrv.SetPublicKey(pk)
	})
	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()
	client.Start(clientCtx)
	defer client.Stop()

	// Wait for worker registration
	for i := 0; i < 20; i++ {
		w, err := clusterMgr.SelectBestWorker()
		if err == nil && w != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	const concurrentUsers = 5
	t.Logf("[STRESS] Starting concurrent benchmark with %d parallel users...", concurrentUsers)

	// Step 1: Pre-register concurrent users
	userTokens := make([]string, concurrentUsers)
	for i := 0; i < concurrentUsers; i++ {
		regBody, _ := json.Marshal(map[string]string{
			"username": fmt.Sprintf("stress_user_%d", i+1),
			"password": "Password123!",
			"email":    fmt.Sprintf("user_%d@attackbox.local", i+1),
		})
		resp, err := http.Post("http://127.0.0.1:18081/api/auth/register", "application/json", bytes.NewReader(regBody))
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("user %d register failed", i+1)
		}
		var authResp struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&authResp)
		resp.Body.Close()
		userTokens[i] = authResp.Token
	}

	// Step 2: Concurrently create sandboxes
	type boxResult struct {
		userIndex int
		boxResp   control.StartSandboxResp
		err       error
		duration  time.Duration
	}

	results := make([]boxResult, concurrentUsers)
	var wg sync.WaitGroup

	startBenchmark := time.Now()
	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reqStart := time.Now()

			reqBody, _ := json.Marshal(control.StartSandboxReq{})

			req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18081/api/sandbox/start", bytes.NewReader(reqBody))
			req.Header.Set("Authorization", "Bearer "+userTokens[idx])
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results[idx] = boxResult{userIndex: idx, err: err, duration: time.Since(reqStart)}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				b, _ := io.ReadAll(resp.Body)
				results[idx] = boxResult{userIndex: idx, err: fmt.Errorf("status %d: %s", resp.StatusCode, string(b)), duration: time.Since(reqStart)}
				return
			}

			var sResp control.StartSandboxResp
			_ = json.NewDecoder(resp.Body).Decode(&sResp)
			results[idx] = boxResult{userIndex: idx, boxResp: sResp, duration: time.Since(reqStart)}
		}(i)
	}
	wg.Wait()
	totalCreationTime := time.Since(startBenchmark)

	t.Logf("[STRESS] All %d sandboxes launched concurrently in %v (avg %.1fms / box)",
		concurrentUsers, totalCreationTime, float64(totalCreationTime.Milliseconds())/float64(concurrentUsers))

	// Verify uniqueness of IPs and SlotIndices
	usedSlots := make(map[int]bool)
	usedIPs := make(map[string]bool)
	for i, res := range results {
		if res.err != nil {
			t.Fatalf("user %d failed to create sandbox: %v", i+1, res.err)
		}
		if usedSlots[res.boxResp.SlotIndex] {
			t.Errorf("duplicate slot index allocated: %d", res.boxResp.SlotIndex)
		}
		usedSlots[res.boxResp.SlotIndex] = true

		if usedIPs[res.boxResp.SandboxIP] {
			t.Errorf("duplicate sandbox IP allocated: %s", res.boxResp.SandboxIP)
		}
		usedIPs[res.boxResp.SandboxIP] = true

		// Check Cgroups v2 on filesystem
		memFile := fmt.Sprintf("/sys/fs/cgroup/attackbox/%s/memory.max", res.boxResp.SandboxID)
		if _, err := os.Stat(memFile); err != nil {
			t.Errorf("cgroup memory.max missing for %s", res.boxResp.SandboxID)
		}
	}

	// Step 3: Concurrently connect PTY WebSocket to all 5 sandboxes
	var ptyWg sync.WaitGroup
	for i := 0; i < concurrentUsers; i++ {
		ptyWg.Add(1)
		go func(idx int) {
			defer ptyWg.Done()
			ticket := results[idx].boxResp.Ticket
			wsURL := fmt.Sprintf("ws://127.0.0.1:18444/ws/cli?ticket=%s", ticket)
			wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Errorf("user %d dial ws failed: %v", idx+1, err)
				return
			}
			defer wsConn.Close()

			tokenStr := fmt.Sprintf("PING_USER_%d_OK\n", idx+1)
			_ = wsConn.WriteMessage(websocket.BinaryMessage, append([]byte{0x00}, []byte(tokenStr)...))

			deadline := time.Now().Add(2 * time.Second)
			matched := false
			for time.Now().Before(deadline) {
				_ = wsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				_, msg, err := wsConn.ReadMessage()
				if err != nil {
					break
				}
				if len(msg) > 1 && strings.Contains(string(msg[1:]), fmt.Sprintf("PING_USER_%d_OK", idx+1)) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("user %d did not receive reflected terminal token", idx+1)
			}
		}(i)
	}
	ptyWg.Wait()
	t.Logf("[STRESS] All %d parallel interactive PTY shells verified successfully!", concurrentUsers)

	// Step 4: Concurrently tear down all 5 sandboxes
	teardownStart := time.Now()
	var stopWg sync.WaitGroup
	for i := 0; i < concurrentUsers; i++ {
		stopWg.Add(1)
		go func(idx int) {
			defer stopWg.Done()
			stopBody, _ := json.Marshal(control.StopSandboxReq{
				SandboxID: results[idx].boxResp.SandboxID,
			})
			req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18081/api/sandbox/stop", bytes.NewReader(stopBody))
			req.Header.Set("Authorization", "Bearer "+userTokens[idx])
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Errorf("user %d stop failed: %v", idx+1, err)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}(i)
	}
	stopWg.Wait()
	totalTeardownTime := time.Since(teardownStart)

	t.Logf("[STRESS] All %d sandboxes destroyed concurrently in %v (avg %.1fms / box, Zero Hang Verified!)",
		concurrentUsers, totalTeardownTime, float64(totalTeardownTime.Milliseconds())/float64(concurrentUsers))

	// Verify all sessions cleared
	if len(sessions.ListAll()) != 0 {
		t.Errorf("lingering active sessions: %d", len(sessions.ListAll()))
	}
}
