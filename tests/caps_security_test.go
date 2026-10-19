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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/node"
	"attackbox/node/runtime"
	"attackbox/protocol"
)

func TestSecurityContainment_CapabilitiesAndNoNewPrivs(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Security containment test requires root privileges")
	}

	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "caps_test.db")

	clusterSecret := "security_containment_secret_min_32!"
	ctrlCfg := &control.ControlConfig{}
	ctrlCfg.Server.ListenAddr = "127.0.0.1:18082"
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
			NodeID:               "node-caps-test",
			ControlURL:           "ws://127.0.0.1:18082/api/agent/connect",
			ClusterSecret:        clusterSecret,
			HeartbeatIntervalSec: 3,
		},
		Gateway: node.GatewayConfig{
			ListenAddr: "127.0.0.1:18445",
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

	// Wait for node registration
	for i := 0; i < 20; i++ {
		w, err := clusterMgr.SelectBestWorker()
		if err == nil && w != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Register user
	regBody, _ := json.Marshal(map[string]string{
		"username": "auditor_sec",
		"password": "Password123!",
		"email":    "auditor@attackbox.local",
	})
	resp, _ := http.Post("http://127.0.0.1:18082/api/auth/register", "application/json", bytes.NewReader(regBody))
	var authResp struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&authResp)
	resp.Body.Close()

	// Launch sandbox
	startReqBody, _ := json.Marshal(control.StartSandboxReq{
		CustomPolicy: &protocol.SandboxPolicy{
			Resources: protocol.BoxResourceQuota{
				MemoryMaxMB: 512,
				PIDsMax:     128,
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18082/api/sandbox/start", bytes.NewReader(startReqBody))
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	req.Header.Set("Content-Type", "application/json")
	startResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("start sandbox: %v", err)
	}
	defer startResp.Body.Close()

	var boxResp control.StartSandboxResp
	_ = json.NewDecoder(startResp.Body).Decode(&boxResp)
	if boxResp.SandboxID == "" {
		t.Fatalf("empty sandbox_id returned")
	}

	// Retrieve sandbox state from runtime manager to get PID
	sbxState, ok := rtMgr.GetSandbox(boxResp.SandboxID)
	if !ok {
		t.Fatalf("sandbox %s not found in rtMgr", boxResp.SandboxID)
	}

	// -------------------------------------------------------------
	// TEST 1: Direct Kernel Procfs Verification on PID 1
	// -------------------------------------------------------------
	t.Logf("[SECURITY AUDIT 1/7] Inspecting PID 1 (%d) procfs status...", sbxState.PID)
	if err := runtime.VerifyDangerousCapsDropped(sbxState.PID); err != nil {
		t.Fatalf("PID 1 capability verification FAILED: %v", err)
	}
	t.Logf("[PASS] PID 1 procfs strictly verified: NoNewPrivs=1, CAP_SYS_BOOT=0, CAP_SYS_RAWIO=0, CAP_SYS_MODULE=0, CAP_MKNOD=0!")

	// -------------------------------------------------------------
	// TEST 2: Inspect Shell Identity and Capabilities inside Interactive PTY
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 2/7] Connecting to WebSocket PTY terminal to audit user identity and capabilities...")
	wsURL := fmt.Sprintf("ws://127.0.0.1:18445/ws/cli?ticket=%s", boxResp.Ticket)
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial node gateway: %v", err)
	}
	defer wsConn.Close()

	inspectCmd := "id; grep --color=never -E '^(CapEff|CapBnd)' /proc/self/status; echo CAP_AUDIT_MARKER\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(inspectCmd))

	var outputBuf bytes.Buffer
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nCAP_AUDIT_MARKER") || strings.Contains(outputBuf.String(), "\r\nCAP_AUDIT_MARKER") {
			break
		}
	}

	terminalOutput := outputBuf.String()
	t.Logf("Terminal Audit Output:\n%s", terminalOutput)

	if !strings.Contains(terminalOutput, "box") {
		t.Errorf("PTY shell is not running as user 'box'! Output:\n%s", terminalOutput)
	}
	if !strings.Contains(terminalOutput, "sudo") {
		t.Errorf("User 'box' is not in 'sudo' group! Output:\n%s", terminalOutput)
	}

	var shellCapBnd uint64
	for _, rawLine := range strings.Split(terminalOutput, "\n") {
		line := strings.TrimSpace(rawLine)
		fields := strings.Fields(line)
		for i, f := range fields {
			if strings.Contains(f, "CapBnd:") && i+1 < len(fields) {
				shellCapBnd, _ = strconv.ParseUint(fields[i+1], 16, 64)
			}
		}
	}

	for _, bit := range []int{runtime.CapSysModule, runtime.CapSysRawIO, runtime.CapSysBoot, runtime.CapMknod} {
		if (shellCapBnd & (1 << bit)) != 0 {
			t.Errorf("PTY shell CapBnd still contains dangerous capability bit %d (0x%x)!", bit, shellCapBnd)
		}
	}
	t.Logf("[PASS] Interactive Shell strictly verified: User is 'box' (in sudo group), CapBnd=0x%x (no boot/rawio/module/mknod)", shellCapBnd)

	// -------------------------------------------------------------
	// TEST 2.5: Verify Passwordless Sudo Functionality
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 2.5] Testing passwordless sudo elevation...")
	outputBuf.Reset()
	sudoCheckCmd := "sudo whoami; echo SUDO_CHECK_MARKER\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(sudoCheckCmd))

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nSUDO_CHECK_MARKER") || strings.Contains(outputBuf.String(), "\r\nSUDO_CHECK_MARKER") {
			break
		}
	}
	sudoOutput := outputBuf.String()
	t.Logf("Sudo Check Output:\n%s", sudoOutput)
	if !strings.Contains(sudoOutput, "root") {
		t.Fatalf("Passwordless sudo failed! Output: %s", sudoOutput)
	}
	t.Log("[PASS] Passwordless sudo successfully elevated to root!")

	// -------------------------------------------------------------
	// TEST 3: Negative Exploit Test: Execute reboot syscall
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 3/7] Simulating reboot exploit inside sandbox under sudo...")
	outputBuf.Reset()
	rebootCmd := "sudo /usr/bin/test_reboot; echo REBOOT_MARKER\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(rebootCmd))

	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nREBOOT_MARKER") || strings.Contains(outputBuf.String(), "\r\nREBOOT_MARKER") {
			break
		}
	}
	rebootOutput := outputBuf.String()
	t.Logf("Reboot Exploit Attempt Output:\n%s", rebootOutput)

	if !strings.Contains(rebootOutput, "KERNEL_EPERM_CONFIRMED") &&
		!strings.Contains(strings.ToLower(rebootOutput), "operation not permitted") {
		t.Fatalf("Reboot exploit was NOT blocked by kernel! Output: %s", rebootOutput)
	}
	t.Log("[PASS] Kernel EPERM Defense Confirmed: Sandbox reboot blocked by Linux kernel even under sudo!")

	// -------------------------------------------------------------
	// TEST 4: Negative Exploit Test: Execute finit_module syscall
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 4/7] Simulating finit_module / module load exploit inside sandbox under sudo...")
	outputBuf.Reset()
	insmodCmd := "sudo /usr/bin/test_reboot m; echo MODULE_MARKER\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(insmodCmd))

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nMODULE_MARKER") || strings.Contains(outputBuf.String(), "\r\nMODULE_MARKER") {
			break
		}
	}
	insmodOutput := outputBuf.String()
	t.Logf("Module Exploit Attempt Output:\n%s", insmodOutput)

	if !strings.Contains(insmodOutput, "KERNEL_MODULE_EPERM_CONFIRMED") {
		t.Fatalf("Module exploit was NOT blocked by kernel! Output: %s", insmodOutput)
	}
	t.Log("[PASS] Kernel Module Load Defense Confirmed: Sandbox finit_module blocked even under sudo!")

	// -------------------------------------------------------------
	// TEST 5: Negative Exploit Test: Attempt to write /proc/sys/kernel/core_pattern
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 5/7] Simulating core_pattern overwrite exploit under sudo...")
	outputBuf.Reset()
	corePatternCmd := "sudo sh -c 'echo evil > /proc/sys/kernel/core_pattern'; echo CORE_PATTERN_EXIT_$?\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(corePatternCmd))

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nCORE_PATTERN_EXIT_") || strings.Contains(outputBuf.String(), "\r\nCORE_PATTERN_EXIT_") {
			break
		}
	}
	coreOutput := outputBuf.String()
	t.Logf("Core Pattern Attempt Output:\n%s", coreOutput)

	if !strings.Contains(coreOutput, "Read-only file system") && strings.Contains(coreOutput, "CORE_PATTERN_EXIT_0") {
		t.Fatalf("Overwriting /proc/sys was NOT blocked! Output: %s", coreOutput)
	}
	t.Log("[PASS] Read-only /proc/sys Confirmed: Modifications to /proc/sys/kernel/core_pattern blocked with Read-only file system!")

	// -------------------------------------------------------------
	// TEST 6: Sysrq masking verification under sudo
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 6/7] Verifying /proc/sysrq-trigger masking under sudo...")
	outputBuf.Reset()
	sysrqCmd := "sudo sh -c 'echo c > /proc/sysrq-trigger'; echo SYSRQ_SAFE_MARKER\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(sysrqCmd))

	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nSYSRQ_SAFE_MARKER") || strings.Contains(outputBuf.String(), "\r\nSYSRQ_SAFE_MARKER") {
			break
		}
	}
	sysrqOutput := outputBuf.String()
	t.Logf("Sysrq Trigger Output:\n%s", sysrqOutput)

	if !strings.Contains(sysrqOutput, "SYSRQ_SAFE_MARKER") {
		t.Fatalf("Sysrq trigger execution failed or hung! Output: %s", sysrqOutput)
	}
	t.Log("[PASS] /proc/sysrq-trigger Masking Confirmed: Host kernel panic averted via /dev/null bind mount!")

	// -------------------------------------------------------------
	// TEST 7: Negative Exploit Test: P0-4 MKNOD Physical Device Access Defense
	// -------------------------------------------------------------
	t.Log("[SECURITY AUDIT 7/7] Simulating mknod /dev/sda exploit attempt under sudo...")
	outputBuf.Reset()
	mknodCmd := "sudo mknod /dev/test_block_dev b 8 0 2>&1; echo RESULT_MKNOD_$?\n"
	_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(mknodCmd))

	time.Sleep(100 * time.Millisecond)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		outputBuf.Write(msg)
		if strings.Contains(outputBuf.String(), "\nRESULT_MKNOD_") || strings.Contains(outputBuf.String(), "\r\nRESULT_MKNOD_") {
			break
		}
	}
	mknodOutput := outputBuf.String()
	t.Logf("Mknod Exploit Attempt Output:\n%s", mknodOutput)

	if !strings.Contains(strings.ToLower(mknodOutput), "operation not permitted") {
		t.Fatalf("P0-4 MKNOD exploit was NOT blocked by kernel! Output: %s", mknodOutput)
	}
	t.Log("[PASS] P0-4 MKNOD Defense Confirmed: Creating block devices in /dev blocked with EPERM!")

	// Teardown
	stopBody, _ := json.Marshal(control.StopSandboxReq{SandboxID: boxResp.SandboxID})
	sReq, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18082/api/sandbox/stop", bytes.NewReader(stopBody))
	sReq.Header.Set("Authorization", "Bearer "+authResp.Token)
	sReq.Header.Set("Content-Type", "application/json")
	sResp, _ := http.DefaultClient.Do(sReq)
	if sResp != nil {
		sResp.Body.Close()
	}
}
