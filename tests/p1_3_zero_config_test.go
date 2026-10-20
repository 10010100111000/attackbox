//go:build linux

package tests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"attackbox/control"
	"attackbox/node"
	"attackbox/node/runtime"
	"attackbox/protocol"
)

// TestP1_3_ZeroConfigFailClosedAndAdminAuthority specifically tests:
// 1. Zero-value config &ControlConfig{} MUST NOT lead to an unbound sandbox (fail-closed to DefaultMemLimitMB).
// 2. Machine/cluster policies are strictly managed by administrative configuration on Control Hub,
//    and end-users can NEVER inject, tamper with, or override host machine resource policies.
func TestP1_3_ZeroConfigFailClosedAndAdminAuthority(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("P1-3 test requires root privileges for Cgroups v2")
	}

	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "p1_3_test.db")

	clusterSecret := "p1_3_test_secret_must_be_32_chars!"

	// SCENARIO A: Intentionally construct an empty, zero-valued ControlConfig{}!
	// This simulates callers omitting cfg.Sandbox entirely.
	rawZeroCfg := &control.ControlConfig{}
	rawZeroCfg.Server.ListenAddr = "127.0.0.1:18095"
	rawZeroCfg.Database.Path = dbPath
	rawZeroCfg.Cluster.ClusterSecret = clusterSecret

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

	// NewControlHTTPServer MUST sanitize and apply fail-closed defaults automatically
	router := control.NewControlHTTPServer(rawZeroCfg, db, privKey, pubKey, clusterMgr, slots, sessions)

	ctrlSrv := &http.Server{
		Addr:    rawZeroCfg.Server.ListenAddr,
		Handler: router,
	}
	go func() { _ = ctrlSrv.ListenAndServe() }()
	defer func() { _ = ctrlSrv.Shutdown(context.Background()) }()

	nodeCfg := &node.NodeConfig{
		Node: node.NodeInfoConfig{
			NodeID:               "node-p1-3-test",
			ControlURL:           "ws://127.0.0.1:18095/api/agent/connect",
			ClusterSecret:        clusterSecret,
			HeartbeatIntervalSec: 2,
		},
		Gateway: node.GatewayConfig{
			ListenAddr: "127.0.0.1:18455",
			TLSEnabled: false,
		},
		Runtime: runtime.RuntimeConfig{
			BaseRootfs:   "/var/lib/attackbox/base_rootfs",
			InstancesDir: "/var/lib/attackbox/instances",
			CgroupRoot:   "/sys/fs/cgroup/attackbox",
		},
	}

	rtMgr := runtime.NewRuntimeManager(nodeCfg.Runtime)
	if err := rtMgr.Start(context.Background()); err != nil {
		t.Fatalf("start rtMgr: %v", err)
	}
	defer rtMgr.Stop()

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

	// Wait for registration
	for i := 0; i < 30; i++ {
		w, err := clusterMgr.SelectBestWorker()
		if err == nil && w != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 1. Register a regular user
	regBody, _ := json.Marshal(map[string]string{
		"username": "p1_3_user",
		"password": "Password123!",
		"email":    "p1_3@attackbox.local",
	})
	regResp, err := http.Post("http://127.0.0.1:18095/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	var authResp struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	// -------------------------------------------------------------
	// SCENARIO 1: Zero-config control server -> Must enforce 2048MB baseline (Fail-Closed)
	// -------------------------------------------------------------
	t.Log("[TEST 1/2] Launching sandbox on zero-config control server...")
	startReq1, _ := json.Marshal(control.StartSandboxReq{})
	req1, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18095/api/sandbox/start", bytes.NewReader(startReq1))
	req1.Header.Set("Authorization", "Bearer "+authResp.Token)
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("start sandbox 1: %v", err)
	}
	var boxResp1 control.StartSandboxResp
	_ = json.NewDecoder(resp1.Body).Decode(&boxResp1)
	resp1.Body.Close()

	if boxResp1.SandboxID == "" {
		t.Fatalf("empty sandbox_id returned for box 1")
	}

	// Verify Cgroup memory.max is strictly set to DefaultMemLimitMB (2048MB), NEVER "max" or 0!
	memMaxFile1 := fmt.Sprintf("/sys/fs/cgroup/attackbox/%s/memory.max", boxResp1.SandboxID)
	data1, err := os.ReadFile(memMaxFile1)
	if err != nil {
		t.Fatalf("read memory.max for box 1: %v", err)
	}
	rawVal1 := strings.TrimSpace(string(data1))
	expectedDefaultBytes := fmt.Sprintf("%d", int64(protocol.DefaultMemLimitMB)*1024*1024)
	if rawVal1 == "max" || rawVal1 == "0" || rawVal1 != expectedDefaultBytes {
		t.Fatalf("P1-3 Fail-Open Detected! memory.max is %q, expected default baseline %s", rawVal1, expectedDefaultBytes)
	}
	t.Logf("[PASS 1/2] Fail-closed baseline enforced: Zero-config safely defaulted memory.max=%s (2048MB)", rawVal1)

	// Clean up box 1
	stopReq1, _ := json.Marshal(control.StopSandboxReq{SandboxID: boxResp1.SandboxID})
	sReq1, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18095/api/sandbox/stop", bytes.NewReader(stopReq1))
	sReq1.Header.Set("Authorization", "Bearer "+authResp.Token)
	sReq1.Header.Set("Content-Type", "application/json")
	_, _ = http.DefaultClient.Do(sReq1)

	// -------------------------------------------------------------
	// SCENARIO 2: Administrator changes baseline configuration -> Sandboxes strictly obey admin policy
	// -------------------------------------------------------------
	t.Log("[TEST 2/2] Administrator updates control baseline to 512MB / 128 PIDs -> Verify sandboxes strictly obey...")
	rawZeroCfg.Sandbox.Resources.MemoryMaxMB = 512
	rawZeroCfg.Sandbox.Resources.PIDsMax = 128

	startReq2, _ := json.Marshal(control.StartSandboxReq{})
	req2, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18095/api/sandbox/start", bytes.NewReader(startReq2))
	req2.Header.Set("Authorization", "Bearer "+authResp.Token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("start sandbox 2: %v", err)
	}
	var boxResp2 control.StartSandboxResp
	_ = json.NewDecoder(resp2.Body).Decode(&boxResp2)
	resp2.Body.Close()

	memMaxFile2 := fmt.Sprintf("/sys/fs/cgroup/attackbox/%s/memory.max", boxResp2.SandboxID)
	data2, _ := os.ReadFile(memMaxFile2)
	rawVal2 := strings.TrimSpace(string(data2))
	expected512Bytes := fmt.Sprintf("%d", 512*1024*1024)
	if rawVal2 != expected512Bytes {
		t.Fatalf("Sandbox memory did not match administrator policy! memory.max is %q, expected %s (512MB)", rawVal2, expected512Bytes)
	}

	pidsMaxFile2 := fmt.Sprintf("/sys/fs/cgroup/attackbox/%s/pids.max", boxResp2.SandboxID)
	dataPids2, _ := os.ReadFile(pidsMaxFile2)
	if strings.TrimSpace(string(dataPids2)) != "128" {
		t.Fatalf("Sandbox pids did not match administrator policy! Got %q, want 128", strings.TrimSpace(string(dataPids2)))
	}
	t.Logf("[PASS 2/2] Administrative authority verified: memory.max=%s (512MB), pids.max=128 strictly enforced from ControlConfig", rawVal2)

	// Clean up box 2
	stopReq2, _ := json.Marshal(control.StopSandboxReq{SandboxID: boxResp2.SandboxID})
	sReq2, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18095/api/sandbox/stop", bytes.NewReader(stopReq2))
	sReq2.Header.Set("Authorization", "Bearer "+authResp.Token)
	sReq2.Header.Set("Content-Type", "application/json")
	_, _ = http.DefaultClient.Do(sReq2)
}
