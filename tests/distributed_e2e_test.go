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
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/node"
	"attackbox/node/runtime"
	"attackbox/protocol"
)

func TestMain(m *testing.M) {
	if runtime.InitDispatcher() {
		return
	}
	os.Exit(m.Run())
}

func TestDistributedE2E_FullLoop(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Distributed E2E test requires root privileges for btrfs and Cgroups v2")
	}

	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "control.db")

	// 1. Initialize Control Hub
	clusterSecret := "test_cluster_secret_32_chars_ok!"
	ctrlCfg := &control.ControlConfig{}
	ctrlCfg.Server.ListenAddr = "127.0.0.1:18080"
	ctrlCfg.Server.CORSAllowedOrigins = []string{"*"}
	ctrlCfg.Database.Path = dbPath
	ctrlCfg.Cluster.ClusterSecret = clusterSecret
	ctrlCfg.Cluster.HeartbeatTimeoutSec = 10
	ctrlCfg.Auth.JWTExpireHours = 1
	ctrlCfg.Auth.TicketTTLSec = 60

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
	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()

	router := control.NewControlHTTPServer(ctrlCfg, db, privKey, pubKey, clusterMgr, slots, sessions)

	ctrlSrv := &http.Server{
		Addr:    ctrlCfg.Server.ListenAddr,
		Handler: router,
	}
	go func() {
		_ = ctrlSrv.ListenAndServe()
	}()
	defer func() {
		_ = ctrlSrv.Shutdown(context.Background())
	}()

	t.Log("[1/7] Control Hub running on :18080")

	// 2. Initialize Node
	nodeCfg := &node.NodeConfig{
		Node: node.NodeInfoConfig{
			NodeID:               "node-e2e-1",
			ControlURL:           "ws://127.0.0.1:18080/api/agent/connect",
			ClusterSecret:        clusterSecret,
			HeartbeatIntervalSec: 3,
		},
		Gateway: node.GatewayConfig{
			ListenAddr: "127.0.0.1:18443",
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

	gwSrv, err := node.NewContainerGatewayServer(nodeCfg.Gateway, rtMgr)
	if err != nil {
		t.Fatalf("init data server: %v", err)
	}
	gwSrv.SetPublicKey(pubKey)
	go func() {
		_ = gwSrv.Start()
	}()
	defer func() {
		_ = gwSrv.Stop(context.Background())
	}()

	client := node.NewNodeControlClient(nodeCfg, rtMgr, func(pk ed25519.PublicKey) {
		gwSrv.SetPublicKey(pk)
	})
	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()
	client.Start(clientCtx)
	defer client.Stop()

	t.Log("[2/7] Node client running, waiting for cluster handshake...")

	// 3. Wait for Node to register in Control Hub
	var workerNode *control.WorkerNode
	for i := 0; i < 20; i++ {
		w, err := clusterMgr.SelectBestWorker()
		if err == nil && w != nil {
			workerNode = w
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if workerNode == nil {
		t.Fatalf("Node failed to register in Control Hub within 4s")
	}
	t.Logf("[3/7] Handshake SUCCESS: Node %s registered (CPUs: %d)", workerNode.NodeID, workerNode.Capabilities.CPUCores)

	// 4. Register a User and Login via REST API
	regBody, _ := json.Marshal(map[string]string{
		"username": "pentester_alice",
		"password": "Password123!",
		"email":    "alice@sec-research.org",
	})
	resp, err := http.Post("http://127.0.0.1:18080/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("register failed (%d): %s", resp.StatusCode, string(body))
	}

	var authResp struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&authResp)
	if authResp.Token == "" {
		t.Fatalf("empty auth token returned")
	}
	t.Log("[4/7] User registered successfully, JWT obtained")

	// 5. Request to start sandbox with Fine-Grained Custom Policy
	startReqBody, _ := json.Marshal(control.StartSandboxReq{
		CustomPolicy: &protocol.SandboxPolicy{
			Resources: protocol.BoxResourceQuota{
				MemoryMaxMB: 512,
				PIDsMax:     128,
				CPUWeight:   150,
			},
			Storage: protocol.BoxStoragePolicy{
				TmpfsTmpSize: "256M",
			},
			Network: protocol.BoxNetworkPolicy{
				EgressMode: "restricted",
			},
		},
	})

	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18080/api/sandbox/start", bytes.NewReader(startReqBody))
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	req.Header.Set("Content-Type", "application/json")

	startResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("start sandbox request: %v", err)
	}
	defer startResp.Body.Close()

	if startResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(startResp.Body)
		t.Fatalf("start sandbox failed (%d): %s", startResp.StatusCode, string(body))
	}

	var boxResp control.StartSandboxResp
	_ = json.NewDecoder(startResp.Body).Decode(&boxResp)
	t.Logf("[5/7] Sandbox launched! ID: %s, Worker: %s, Ticket: %s..., IP: %s",
		boxResp.SandboxID, boxResp.WorkerID, boxResp.Ticket[:16], boxResp.SandboxIP)

	// 6. Verify Cgroups v2 on host matches Control's fine-grained policy!
	memMaxPath := fmt.Sprintf("/sys/fs/cgroup/attackbox/%s/memory.max", boxResp.SandboxID)
	memData, err := os.ReadFile(memMaxPath)
	if err != nil {
		t.Fatalf("failed to read cgroup memory.max: %v", err)
	}
	expectedMemBytes := fmt.Sprintf("%d", 512*1024*1024)
	if strings.TrimSpace(string(memData)) != expectedMemBytes {
		t.Errorf("Cgroups memory.max mismatch! Got %s, want %s", strings.TrimSpace(string(memData)), expectedMemBytes)
	}

	pidsMaxPath := fmt.Sprintf("/sys/fs/cgroup/attackbox/%s/pids.max", boxResp.SandboxID)
	pidsData, err := os.ReadFile(pidsMaxPath)
	if err != nil {
		t.Fatalf("failed to read cgroup pids.max: %v", err)
	}
	if strings.TrimSpace(string(pidsData)) != "128" {
		t.Errorf("Cgroups pids.max mismatch! Got %s, want 128", strings.TrimSpace(string(pidsData)))
	}
	t.Logf("[6/7] Kernel Verification PASS: Cgroups v2 strictly enforced memory.max=512MB, pids.max=128!")

	// 7. Connect to Node Gateway Server via WebSocket using issued Ticket
	wsURL := fmt.Sprintf("ws://127.0.0.1:18443/ws/cli?ticket=%s", boxResp.Ticket)
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial node gateway server: %v", err)
	}
	defer wsConn.Close()

	testPayload := []byte("echo CONTROL_DISPATCH_E2E_VERIFIED\n")
	frame := append([]byte{0x00}, testPayload...)
	if err := wsConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("write pty input: %v", err)
	}

	matched := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		if len(msg) > 1 && strings.Contains(string(msg[1:]), "CONTROL_DISPATCH_E2E_VERIFIED") {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("did not receive reflected command output from PTY bridge")
	}
	t.Log("[7/7] Interactive PTY Bridge Verified: Bidirectional shell I/O working!")

	// 8. Stop Sandbox via Control Hub
	stopReqBody, _ := json.Marshal(control.StopSandboxReq{
		SandboxID: boxResp.SandboxID,
	})
	stopReq, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18080/api/sandbox/stop", bytes.NewReader(stopReqBody))
	stopReq.Header.Set("Authorization", "Bearer "+authResp.Token)
	stopReq.Header.Set("Content-Type", "application/json")

	stopResp, err := http.DefaultClient.Do(stopReq)
	if err != nil {
		t.Fatalf("stop sandbox request: %v", err)
	}
	defer stopResp.Body.Close()

	if stopResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(stopResp.Body)
		t.Fatalf("stop sandbox failed (%d): %s", stopResp.StatusCode, string(body))
	}

	var stopData control.StopSandboxResp
	_ = json.NewDecoder(stopResp.Body).Decode(&stopData)
	if !stopData.Success {
		t.Fatalf("stopData reported success=false")
	}

	t.Logf("Sandbox stopped cleanly! Duration: %ds", stopData.DurationSeconds)
}
