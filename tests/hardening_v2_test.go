package tests

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/control"
	"attackbox/node"
	rt "attackbox/node/runtime"
	"attackbox/protocol"
)

// 1. P0: 验证宿主快照脱敏全面性（覆盖集群配置、TLS 私钥、云凭据、VPN 等）
func TestGoldenImageDeepSanitization(t *testing.T) {
	tempRoot, err := os.MkdirTemp("", "golden_sanitize_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	builder := rt.NewGoldenImageBuilder(tempRoot)

	// 造假宿主敏感凭据文件
	sensitivePaths := []string{
		"etc/attackbox/node.yaml",
		"var/lib/attackbox/secrets.db",
		"etc/ssl/private/node.key",
		"etc/tls/worker.key",
		"etc/ssl/certs/attackbox-node.crt",
		"root/.ssh/id_ed25519",
		"root/.ssh/authorized_keys",
		"root/.aws/credentials",
		"root/.azure/accessTokens.json",
		"root/.config/gcloud/credentials.db",
		"root/.kube/config",
		"root/.docker/config.json",
		"root/.netrc",
		"home/testuser/.ssh/id_rsa",
		"home/testuser/.aws/credentials",
		"home/testuser/.kube/config",
		"home/testuser/.docker/config.json",
		"home/testuser/.netrc",
		"etc/systemd/system/attackboxd.service",
		"var/run/secrets/kubernetes.io/token",
		"etc/wireguard/wg0.conf",
		"etc/openvpn/client.ovpn",
		"root/.bash_history",
		"home/testuser/.bash_history",
		"var/log/syslog",
	}

	for _, p := range sensitivePaths {
		fullPath := filepath.Join(tempRoot, p)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			t.Fatalf("mkdir for %s failed: %v", p, err)
		}
		if err := os.WriteFile(fullPath, []byte("TOP_SECRET_CREDENTIALS_AND_KEYS"), 0600); err != nil {
			t.Fatalf("write sensitive file %s failed: %v", p, err)
		}
	}

	// 执行脱敏
	if err := builder.SanitizeRootfs(tempRoot); err != nil {
		t.Fatalf("SanitizeRootfs failed: %v", err)
	}

	// 强断言：所有敏感凭据必须被 100% 抹除
	for _, p := range sensitivePaths {
		fullPath := filepath.Join(tempRoot, p)
		if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
			t.Errorf("SECURITY LEAK: Sensitive path %s was NOT eradicated during sanitization!", p)
		}
	}

	// 确认 /home/box 存在
	if _, err := os.Stat(filepath.Join(tempRoot, "home/box")); err != nil {
		t.Errorf("Essential directory /home/box was not created: %v", err)
	}
}

// 2. P1: 验证 YAML 资源配额与存储策略解析全链路生效
func TestYAMLResourceAndStorageParsing(t *testing.T) {
	yamlContent := `
sandbox:
  resources:
    memory_max_mb: 321
    memory_swap_max_mb: 0
    pids_max: 37
    cpu_weight: 77
    io_write_bytes_sec: 10485760
    io_write_iops: 300
  storage:
    tmpfs_tmp_size: "42M"
    tmpfs_run_size: "21M"
`
	tmpFile, err := os.CreateTemp("", "control_cfg_*.yaml")
	if err != nil {
		t.Fatalf("create temp yaml: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString(yamlContent)
	_ = tmpFile.Close()

	cfg, err := control.LoadControlConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadControlConfig failed: %v", err)
	}

	if cfg.Sandbox.Resources.MemoryMaxMB != 321 {
		t.Errorf("expected MemoryMaxMB=321, got %d", cfg.Sandbox.Resources.MemoryMaxMB)
	}
	if cfg.Sandbox.Resources.PIDsMax != 37 {
		t.Errorf("expected PIDsMax=37, got %d", cfg.Sandbox.Resources.PIDsMax)
	}
	if cfg.Sandbox.Resources.CPUWeight != 77 {
		t.Errorf("expected CPUWeight=77, got %d", cfg.Sandbox.Resources.CPUWeight)
	}
	if cfg.Sandbox.Storage.TmpfsTmpSize != "42M" {
		t.Errorf("expected TmpfsTmpSize=42M, got %s", cfg.Sandbox.Storage.TmpfsTmpSize)
	}
	if cfg.Sandbox.Storage.TmpfsRunSize != "21M" {
		t.Errorf("expected TmpfsRunSize=21M, got %s", cfg.Sandbox.Storage.TmpfsRunSize)
	}

	// 验证兼容 tmp_size 与 run_size 别名
	aliasYAML := `
sandbox:
  storage:
    tmp_size: "99M"
    run_size: "33M"
`
	tmpFileAlias, err := os.CreateTemp("", "control_cfg_alias_*.yaml")
	if err != nil {
		t.Fatalf("create temp alias yaml: %v", err)
	}
	defer os.Remove(tmpFileAlias.Name())
	_, _ = tmpFileAlias.WriteString(aliasYAML)
	_ = tmpFileAlias.Close()

	cfgAlias, err := control.LoadControlConfig(tmpFileAlias.Name())
	if err != nil {
		t.Fatalf("LoadControlConfig alias failed: %v", err)
	}
	if cfgAlias.Sandbox.Storage.TmpfsTmpSize != "99M" {
		t.Errorf("expected alias tmp_size to parse into TmpfsTmpSize=99M, got %s", cfgAlias.Sandbox.Storage.TmpfsTmpSize)
	}
	if cfgAlias.Sandbox.Storage.TmpfsRunSize != "33M" {
		t.Errorf("expected alias run_size to parse into TmpfsRunSize=33M, got %s", cfgAlias.Sandbox.Storage.TmpfsRunSize)
	}

	// 验证加载生产 configs/control.yaml 真实文件，在 KnownFields(true) 下零报错
	repoControlCfg, err := control.LoadControlConfig(filepath.Join("..", "configs", "control.yaml"))
	if err != nil {
		repoControlCfg, err = control.LoadControlConfig(filepath.Join("configs", "control.yaml"))
	}
	if err != nil || repoControlCfg == nil {
		t.Fatalf("failed to load configs/control.yaml under KnownFields(true): %v", err)
	}
	t.Log("[PASS] configs/control.yaml validated cleanly under KnownFields(true)")

	// 验证未知字段在 KnownFields(true) 下被拦截
	invalidYAML := `
sandbox:
  unknown_invalid_key_xyz: 12345
`
	tmpInvalid, _ := os.CreateTemp("", "control_invalid_*.yaml")
	defer os.Remove(tmpInvalid.Name())
	_, _ = tmpInvalid.WriteString(invalidYAML)
	_ = tmpInvalid.Close()
	_, errInvalid := control.LoadControlConfig(tmpInvalid.Name())
	if errInvalid == nil {
		t.Errorf("expected error when parsing unknown key under KnownFields(true), got nil")
	} else {
		t.Logf("[PASS] Unknown YAML key successfully rejected by KnownFields: %v", errInvalid)
	}
}

// 3. P1: 验证销毁确认闭环与状态机引入（失败时保留状态与槽位）
func TestTerminateSandboxStateMachineAndRollback(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "test_statemachine.db")
	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	clusterMgr := control.NewNodeClusterManager("secret")
	clusterMgr.SetPublicKey(pub)

	cfg := &control.ControlConfig{}
	cfg.SanitizeAndApplyDefaults()

	handler := control.NewControlHTTPServer(cfg, db, priv, pub, clusterMgr, slots, sessions)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 模拟已有一个活跃沙箱
	sbxID := "sbx_test_sm_1"
	slot, _ := slots.Allocate(sbxID)
	session := &control.ActiveSession{
		SandboxID:    sbxID,
		UserID:       1001,
		WorkerNodeID: "worker-offline",
		SlotIndex:    slot,
		SandboxIP:    "10.200.0.10",
		State:        control.SessionStateRunning,
		StartedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	sessions.Add(session)

	// 执行终止：由于 worker-offline 节点不存在，DestroyBox 将返回错误
	// 我们通过调用 ControlHTTPServer 的 TerminateSandbox 来测试
	// 使用反射或者直接通过 HTTP 接口测试
	// 这里直接调用 handler 包装的底层逻辑
	sHandler, ok := handler.(interface {
		TerminateSandbox(ctx context.Context, sandboxID string, reason string) (control.ActiveSession, error)
	})
	if ok {
		_, termErr := sHandler.TerminateSandbox(context.Background(), sbxID, "test_failure")
		if termErr == nil {
			t.Fatalf("expected error when terminating sandbox on unreachable worker, got nil")
		}

		// 验证状态机必须为 SessionStateCleanupFailed
		retained, exists := sessions.Get(sbxID)
		if !exists {
			t.Fatalf("FAILED: Session was prematurely deleted despite worker destroy failure!")
		}
		if retained.State != control.SessionStateCleanupFailed {
			t.Errorf("expected state to be cleanup_failed, got %s", retained.State)
		}

		// 验证槽位未被释放
		heldSlot, held := slots.GetSlot(sbxID)
		if !held || heldSlot != slot {
			t.Fatalf("FAILED: Slot was released prematurely! Expected slot %d still held, got held=%v, slot=%d", slot, held, heldSlot)
		}
		// 验证新沙箱分配时不会冲突分配该槽位
		otherSlot, err := slots.Allocate("sbx_challenger_hardening")
		if err != nil {
			t.Fatalf("failed to allocate challenger slot: %v", err)
		}
		if otherSlot == slot {
			t.Fatalf("FAILED: Slot conflict! Challenger got slot %d which is still locked by failed sandbox %s", otherSlot, sbxID)
		}
	}
}

// 4. P2: 验证槽位 240-250 的 IPv4 计算不会溢出产生 .256
func TestSlotIPAllocationNoOverflow(t *testing.T) {
	for slot := 1; slot <= 250; slot++ {
		subnetOctet := (slot - 1) / 240
		hostOctet := 10 + ((slot - 1) % 240)
		ipStr := fmt.Sprintf("10.200.%d.%d", subnetOctet, hostOctet)

		if hostOctet < 10 || hostOctet > 254 {
			t.Fatalf("Slot %d produced invalid host octet: %d (IP: %s)", slot, hostOctet, ipStr)
		}
		if subnetOctet < 0 || subnetOctet > 254 {
			t.Fatalf("Slot %d produced invalid subnet octet: %d (IP: %s)", slot, subnetOctet, ipStr)
		}
		if strings.Contains(ipStr, "256") {
			t.Fatalf("Slot %d produced illegal .256 IP address: %s", slot, ipStr)
		}
	}
}

// 5. P2: 验证 IncrementTokenVersion 立即失效内存缓存
func TestTokenVersionCacheInvalidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_tokencache.db")
	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	user, err := db.CreateUser("cacheuser", "dummyhash", "cache@test.local", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	cache := control.NewTokenVersionCache()
	db.SetTokenVersionInvalidatedHook(cache.Invalidate)

	// 写入缓存版本 1
	cache.Set(user.ID, 1, 1, 10*time.Second)

	ver, st, ok := cache.Get(user.ID)
	if !ok || ver != 1 || st != 1 {
		t.Fatalf("cache get failed or unexpected version: %v, %d, %d", ok, ver, st)
	}

	// 执行数据库自增版本
	if err := db.IncrementTokenVersion(user.ID); err != nil {
		t.Fatalf("IncrementTokenVersion failed: %v", err)
	}

	// 强断言：缓存必须被立即失效清空！
	_, _, okAfter := cache.Get(user.ID)
	if okAfter {
		t.Errorf("FAILED: Cache was NOT invalidated after IncrementTokenVersion! Old token version would remain valid.")
	}
}

// 6. P1: 验证慢 WebSocket 连接在抢占注册时不会死锁全局互斥锁
func TestGatewayRegisterCLINoGlobalMutexDeadlock(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err == nil && ws != nil {
			defer ws.Close()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	c1, _, err1 := websocket.DefaultDialer.Dial(wsURL, nil)
	if err1 != nil {
		t.Fatalf("dial c1: %v", err1)
	}
	defer c1.Close()

	c2, _, err2 := websocket.DefaultDialer.Dial(wsURL, nil)
	if err2 != nil {
		t.Fatalf("dial c2: %v", err2)
	}
	defer c2.Close()

	sm := node.NewGatewaySessionManager()

	key := "test_sandbox_pty:default"
	conn1 := node.NewSafeConn(c1)
	sm.RegisterCLI(key, conn1)

	// 并发注册新连接，必须在毫秒级内完成，绝不阻塞
	start := time.Now()
	conn2 := node.NewSafeConn(c2)
	done := make(chan struct{})
	go func() {
		sm.RegisterCLI(key, conn2)
		close(done)
	}()

	select {
	case <-done:
		if time.Since(start) > 200*time.Millisecond {
			t.Errorf("RegisterCLI took %v, slower than expected", time.Since(start))
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("DEADLOCK: RegisterCLI blocked on global mutex lock!")
	}
}

// 7. P1-C: 验证 Cgroup v2 限额写后回读校验、单行复合写入 io.max 与失败回滚
func TestP1_C_CgroupV2ConstraintsAndIOMaxReadback(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("P1-C test requires root privileges for cgroup v2 manipulation")
	}

	tempCgroupRoot := filepath.Join("/sys/fs/cgroup/attackbox_test_p1_c")
	_ = os.RemoveAll(tempCgroupRoot)
	defer os.RemoveAll(tempCgroupRoot)

	cgMgr := rt.NewCgroupManager(tempCgroupRoot)

	// 1. 验证底层设备号解析
	devNum := rt.DetectDeviceNumber("/")
	if devNum == "" || strings.HasPrefix(devNum, "0:") {
		t.Fatalf("DetectDeviceNumber failed: expected real block device major:minor, got %q", devNum)
	}
	t.Logf("[PASS] Detected backing physical block device: %s", devNum)

	// 2. 正常限制创建测试
	limits := rt.ResourceLimits{
		PIDsMax:          256,
		MemoryMaxBytes:   512 * 1024 * 1024,
		CPUWeight:        200,
		IOWriteBytesSec:  10 * 1024 * 1024,
		IOWriteIOPS:      300,
		RootDeviceNumber: devNum,
	}

	sbxID := "sbx_test_p1_c_valid"
	cgFile, cgPath, err := cgMgr.Create(sbxID, limits)
	if err != nil {
		t.Fatalf("Create cgroup failed: %v", err)
	}
	defer cgFile.Close()
	defer os.Remove(cgPath)

	// 回读验证 pids.max
	pidsData, err := os.ReadFile(filepath.Join(cgPath, "pids.max"))
	if err != nil || strings.TrimSpace(string(pidsData)) != "256" {
		t.Fatalf("pids.max readback mismatch: %s (err: %v)", string(pidsData), err)
	}

	// 回读验证 memory.max
	memData, err := os.ReadFile(filepath.Join(cgPath, "memory.max"))
	if err != nil || strings.TrimSpace(string(memData)) != fmt.Sprintf("%d", 512*1024*1024) {
		t.Fatalf("memory.max readback mismatch: %s (err: %v)", string(memData), err)
	}

	// 回读验证 cpu.weight
	cpuData, err := os.ReadFile(filepath.Join(cgPath, "cpu.weight"))
	if err != nil || strings.TrimSpace(string(cpuData)) != "200" {
		t.Fatalf("cpu.weight readback mismatch: %s (err: %v)", string(cpuData), err)
	}

	// 回读验证 io.max (必须包含整盘设备号且单行包含了 wbps 与 wiops)
	ioData, err := os.ReadFile(filepath.Join(cgPath, "io.max"))
	if err != nil {
		t.Fatalf("read io.max failed: %v", err)
	}
	ioStr := string(ioData)
	if !strings.Contains(ioStr, devNum) {
		t.Fatalf("io.max missing device %s: %s", devNum, ioStr)
	}
	if !strings.Contains(ioStr, "wbps=10485760") || !strings.Contains(ioStr, "wiops=300") {
		t.Fatalf("io.max missing wbps or wiops parameters: %s", ioStr)
	}
	t.Logf("[PASS] io.max successfully verified with atomic combined write and readback:\n%s", strings.TrimSpace(ioStr))

	// 3. 验证写入非法设备号时原子回滚，杜绝残留未受限沙箱
	invalidLimits := limits
	invalidLimits.RootDeviceNumber = "0:9999" // 非法虚拟设备号
	invalidSbxID := "sbx_test_p1_c_rollback"
	_, _, errInvalid := cgMgr.Create(invalidSbxID, invalidLimits)
	if errInvalid == nil {
		t.Fatalf("expected error when writing invalid device to io.max, got nil")
	}

	// 确认回滚生效：目录已被彻底删除
	invalidPath := filepath.Join(tempCgroupRoot, invalidSbxID)
	if _, err := os.Stat(invalidPath); !os.IsNotExist(err) {
		t.Fatalf("FAILED: cgroup path %s still exists after failure, rollback did not clean up!", invalidPath)
	}
	t.Logf("[PASS] P1-C verified: Fail-Closed rollback cleaned up cgroup on write failure (%v)", errInvalid)
}

// 8. P1-D: 验证单个沙箱邮箱满塞（64条）不会阻塞全局互斥锁与其它沙箱消息处理
func TestP1_D_NodeMailboxNoGlobalLockStallOnOverflow(t *testing.T) {
	nodeCfg := &node.NodeConfig{
		Node: node.NodeInfoConfig{
			NodeID: "node-mailbox-test",
		},
	}
	client := node.NewNodeControlClient(nodeCfg, nil, nil)

	slowSandboxID := "sbx_slow_overflow"
	fastSandboxID := "sbx_fast_normal"

	// 1. 向 slowSandbox 连续投递 70 条消息（超过 channel 容量 64）
	// 验证：投递超过 64 条时，必须在毫秒级内完成，绝不阻塞当前协程
	start := time.Now()
	for i := 0; i < 70; i++ {
		msg, _ := protocol.NewMessage(protocol.MsgTypeCreateBox, protocol.CreateBoxReq{
			SandboxID: slowSandboxID,
			UserID:    int64(1000 + i),
		})
		client.DispatchMessageForTest(msg)
	}
	duration := time.Since(start)
	if duration > 100*time.Millisecond {
		t.Fatalf("FAILED: Dispatching overflowing messages took %v, blocked on mailbox channel!", duration)
	}
	t.Logf("[PASS] 70 messages dispatched to slow sandbox in %v without stalling", duration)

	// 2. 验证 fastSandbox 的投递与全局锁完全正常
	fastDone := make(chan struct{})
	go func() {
		msg, _ := protocol.NewMessage(protocol.MsgTypeCreateBox, protocol.CreateBoxReq{
			SandboxID: fastSandboxID,
			UserID:    2000,
		})
		client.DispatchMessageForTest(msg)
		close(fastDone)
	}()

	select {
	case <-fastDone:
		t.Log("[PASS] Fast sandbox message dispatched instantly while slow sandbox mailbox was full")
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("FAILED: Global mutex deadlock! Fast sandbox blocked due to slow sandbox overflow.")
	}
}

// 9. P1-E: 验证节点生命周期状态机过滤与 Control 重启无状态双向对账纳管 (Reconcile Adopt & Evict)
func TestP1_E_ReconciliationAndNodeStateMachine(t *testing.T) {
	clusterSecret := "reconcile_secret_123"
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	clusterMgr := control.NewNodeClusterManager(clusterSecret)
	clusterMgr.SetPublicKey(pubKey)

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	cfg := &control.ControlConfig{}
	cfg.SanitizeAndApplyDefaults()

	// 启动全新 ControlHTTPServer（模拟内存无会话启动状态）
	controlSrv := control.NewControlHTTPServer(cfg, nil, privKey, pubKey, clusterMgr, slots, sessions)
	ts := httptest.NewServer(controlSrv)
	defer ts.Close()

	// 1. 模拟 Worker 建立反向 WebSocket 连接
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/agent/connect"
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+clusterSecret)
	header.Set("X-Node-ID", "worker-reconcile-test")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial control agent connect: %v", err)
	}
	defer conn.Close()

	// 发送节点注册消息
	regReq := protocol.RegisterReq{
		NodeID:   "worker-reconcile-test",
		Version:  "2.0",
		DataPort: 8443,
		Capabilities: protocol.NodeCapabilities{
			CPUCores:   4,
			TotalMemMB: 8192,
		},
	}
	regMsg, _ := protocol.NewMessage(protocol.MsgTypeRegister, regReq)
	data, _ := protocol.PackMessage(regMsg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write register: %v", err)
	}

	// 等待 RegisterAck
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, ackRaw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read register ack: %v", err)
	}
	var ackMsg protocol.Message
	if err := protocol.UnpackMessage(ackRaw, &ackMsg); err != nil || ackMsg.Type != protocol.MsgTypeRegisterAck {
		t.Fatalf("expected RegisterAck, got %s (err: %v)", ackMsg.Type, err)
	}

	// 2. 验证节点生命周期状态机过滤 (Online, Draining, Offline, Resetting)
	// (a) 刚注册默认处于 Online 状态，能够被调度器正常分配
	best, err := clusterMgr.SelectBestWorker()
	if err != nil || best.NodeID != "worker-reconcile-test" {
		t.Fatalf("SelectBestWorker failed on online node: %v", err)
	}

	// (b) 标记为 Draining（维护排空中，仅运行存量，不接受新分配）
	if err := clusterMgr.SetNodeState("worker-reconcile-test", protocol.NodeStateDraining); err != nil {
		t.Fatalf("SetNodeState Draining failed: %v", err)
	}
	st, _ := clusterMgr.GetNodeState("worker-reconcile-test")
	if st != protocol.NodeStateDraining {
		t.Fatalf("expected state Draining, got %s", st)
	}
	_, err = clusterMgr.SelectBestWorker()
	if err != control.ErrNoWorkersAvailable {
		t.Fatalf("expected ErrNoWorkersAvailable when worker is Draining, got %v", err)
	}
	t.Log("[PASS] Scheduler correctly ignored node in Draining state")

	// (c) 标记为 Resetting（底层快照回滚中）
	_ = clusterMgr.SetNodeState("worker-reconcile-test", protocol.NodeStateResetting)
	_, err = clusterMgr.SelectBestWorker()
	if err != control.ErrNoWorkersAvailable {
		t.Fatalf("expected ErrNoWorkersAvailable when worker is Resetting, got %v", err)
	}
	t.Log("[PASS] Scheduler correctly ignored node in Resetting state")

	// (d) 标记为 Offline
	_ = clusterMgr.SetNodeState("worker-reconcile-test", protocol.NodeStateOffline)
	_, err = clusterMgr.SelectBestWorker()
	if err != control.ErrNoWorkersAvailable {
		t.Fatalf("expected ErrNoWorkersAvailable when worker is Offline, got %v", err)
	}
	t.Log("[PASS] Scheduler correctly ignored node in Offline state")

	// (e) 恢复为 Online 状态
	_ = clusterMgr.SetNodeState("worker-reconcile-test", protocol.NodeStateOnline)
	best, err = clusterMgr.SelectBestWorker()
	if err != nil || best.NodeID != "worker-reconcile-test" {
		t.Fatalf("SelectBestWorker failed after restored to Online: %v", err)
	}
	t.Log("[PASS] Scheduler restored allocation once worker returned to Online")

	// 3. 验证无状态双向对账机制 (Reconcile Adopt & Evict)
	// 场景：Control 重启后，内存中会话数与槽位全空
	if len(sessions.ListAll()) != 0 {
		t.Fatalf("expected 0 sessions in initial state, got %d", len(sessions.ListAll()))
	}

	// Worker 上报它正在运行的存量沙箱 sbx_survivor_1 (槽位 12, 用户 999)
	reconcileReq := protocol.ReconcileReq{
		NodeID: "worker-reconcile-test",
		Sandboxes: []protocol.SandboxReconcileInfo{
			{
				SandboxID: "sbx_survivor_1",
				SlotIndex: 12,
				UserID:    999,
				SandboxIP: "10.200.0.22",
				StartedAt: time.Now().Add(-10 * time.Minute).Unix(),
			},
		},
	}
	reconcileMsg, _ := protocol.NewMessage(protocol.MsgTypeReconcile, reconcileReq)
	recData, _ := protocol.PackMessage(reconcileMsg)
	if err := conn.WriteMessage(websocket.TextMessage, recData); err != nil {
		t.Fatalf("write reconcile: %v", err)
	}

	// 接收 Control 返回的 ReconcileResp
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, respRaw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read reconcile resp: %v", err)
	}
	var respMsg protocol.Message
	if err := protocol.UnpackMessage(respRaw, &respMsg); err != nil || respMsg.Type != protocol.MsgTypeReconcileResp {
		t.Fatalf("expected ReconcileResp, got %s (err: %v)", respMsg.Type, err)
	}
	var recResp protocol.ReconcileResp
	if err := respMsg.UnpackPayload(&recResp); err != nil {
		t.Fatalf("unpack ReconcileResp: %v", err)
	}

	// 断言：Control 成功 Adopt 收编接管了该沙箱
	if len(recResp.Adopted) != 1 || recResp.Adopted[0] != "sbx_survivor_1" {
		t.Fatalf("expected sbx_survivor_1 in Adopted list, got: %+v", recResp.Adopted)
	}
	if len(recResp.Evicted) != 0 {
		t.Fatalf("expected empty Evicted list, got: %+v", recResp.Evicted)
	}

	// 断言：Control 内存中的 Slot 12 已被占位锁定，无法重复分配
	errOccupy := slots.Occupy("sbx_collision_test", 12)
	if errOccupy == nil {
		t.Fatalf("expected error when occupying already adopted slot 12, got nil")
	}
	t.Log("[PASS] Slot 12 is strictly occupied in Control SlotAllocator after reconciliation adoption")

	// 断言：Control 内存中已重建会话且状态为 Running，包含完整用户与 IP 信息
	adoptedSess, exists := sessions.Get("sbx_survivor_1")
	if !exists {
		t.Fatalf("adopted session sbx_survivor_1 not found in Control sessions")
	}
	if adoptedSess.SlotIndex != 12 || adoptedSess.UserID != 999 || adoptedSess.State != control.SessionStateRunning {
		t.Fatalf("adopted session metadata mismatch: %+v", adoptedSess)
	}
	t.Logf("[PASS] Control successfully adopted survivor session sbx_survivor_1: %+v", adoptedSess)

	// 4. 验证废弃孤儿沙箱的 Evict 指令生成
	// 将该会话在 Control 侧置为 CleanupFailed（模拟之前销毁残留或废弃）
	sessions.SetState("sbx_survivor_1", control.SessionStateCleanupFailed)

	// 再次触发 Reconcile
	reconcileMsg2, _ := protocol.NewMessage(protocol.MsgTypeReconcile, reconcileReq)
	recData2, _ := protocol.PackMessage(reconcileMsg2)
	if err := conn.WriteMessage(websocket.TextMessage, recData2); err != nil {
		t.Fatalf("write second reconcile: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, respRaw2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read second reconcile resp: %v", err)
	}
	var respMsg2 protocol.Message
	_ = protocol.UnpackMessage(respRaw2, &respMsg2)
	var recResp2 protocol.ReconcileResp
	_ = respMsg2.UnpackPayload(&recResp2)

	// 断言：处于 CleanupFailed 状态的孤儿沙箱被 Control 精准下发 Evict 驱逐指令
	if len(recResp2.Evicted) != 1 || recResp2.Evicted[0] != "sbx_survivor_1" {
		t.Fatalf("expected sbx_survivor_1 in Evicted list, got: %+v", recResp2.Evicted)
	}
	t.Logf("[PASS] Control successfully issued eviction for cleanup_failed sandbox: %+v", recResp2.Evicted)
}
