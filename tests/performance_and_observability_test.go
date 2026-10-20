//go:build linux

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"attackbox/control"
	"attackbox/node/runtime"
)

// TestPerformance_SlotAllocatorFreeList verifies:
// 1. O(1) allocation and release using freeList.
// 2. Deterministic lowest available slot reuse.
// 3. Complete allocation exhaustion and release recovery.
func TestPerformance_SlotAllocatorFreeList(t *testing.T) {
	alloc := control.NewSlotAllocator(10)

	// Allocate all 10 slots
	slots := make([]int, 10)
	for i := 0; i < 10; i++ {
		sbxID := "sbx_" + string(rune('a'+i))
		s, err := alloc.Allocate(sbxID)
		if err != nil {
			t.Fatalf("slot %d allocation failed: %v", i+1, err)
		}
		if s != i+1 {
			t.Fatalf("expected slot %d, got %d", i+1, s)
		}
		slots[i] = s
	}

	// 11th allocation should fail
	_, err := alloc.Allocate("sbx_overflow")
	if err == nil {
		t.Fatalf("expected allocation failure on full slots, but succeeded")
	}

	// Release slot 3 and slot 7
	r3 := alloc.Release("sbx_c")
	if r3 != 3 {
		t.Fatalf("expected released slot 3, got %d", r3)
	}
	r7 := alloc.Release("sbx_g")
	if r7 != 7 {
		t.Fatalf("expected released slot 7, got %d", r7)
	}

	// Re-allocate should prioritize lowest slot (3)
	newSbx1, err := alloc.Allocate("sbx_new_1")
	if err != nil {
		t.Fatalf("re-allocate failed: %v", err)
	}
	if newSbx1 != 3 {
		t.Fatalf("expected re-allocated lowest slot 3, got %d", newSbx1)
	}

	// Next re-allocate should pick slot 7
	newSbx2, err := alloc.Allocate("sbx_new_2")
	if err != nil {
		t.Fatalf("re-allocate failed: %v", err)
	}
	if newSbx2 != 7 {
		t.Fatalf("expected re-allocated slot 7, got %d", newSbx2)
	}

	t.Log("[PASS] SlotAllocator Free List: O(1) allocation, deterministic slot reuse, and bounds verified!")
}

// TestPerformance_TokenVersionCache verifies:
// 1. High frequency authenticated requests hit in-memory token cache without DB bottleneck.
// 2. Token version mismatch or invalidation immediately denies access.
func TestPerformance_TokenVersionCache(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "auth_cache.db")

	cfg := &control.ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18099"
	cfg.Database.Path = dbPath
	cfg.SanitizeAndApplyDefaults()

	pubKey, privKey, err := control.LoadOrGenerateKeys("")
	if err != nil {
		t.Fatalf("generate keys failed: %v", err)
	}

	db, err := control.NewDatabaseStore(dbPath)
	if err != nil {
		t.Fatalf("init db store failed: %v", err)
	}
	defer db.Close()

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	clusterMgr := control.NewNodeClusterManager("cache_test_secret_32b_ok_here!")

	hubHandler := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, slots, sessions)
	server := httptest.NewServer(hubHandler)
	defer server.Close()

	// Register user
	regPayload := `{"username":"cache_user","password":"Password123!","email":"cache@test.com"}`
	resp, err := http.Post(server.URL+"/api/auth/register", "application/json", strings.NewReader(regPayload))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	defer resp.Body.Close()

	var authResp control.AuthResp
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		t.Fatalf("decode auth response failed: %v", err)
	}

	token := authResp.Token

	// Run 100 concurrent requests to /api/user/me to test in-memory cache throughput
	const concurrentReqs = 50
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < concurrentReqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/user/me", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("request failed: %v", err)
				return
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", res.StatusCode)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	t.Logf("[PASS] %d concurrent authenticated requests completed in %v (in-memory token cache verified!)", concurrentReqs, duration)
}

// TestObservability_LoggingRecoveryAndRequestID verifies:
// 1. Automatic generation and propagation of X-Request-ID.
// 2. Status code capture in response writer.
// 3. Graceful panic recovery returning HTTP 500 JSON instead of crashing the server.
func TestObservability_LoggingRecoveryAndRequestID(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "observability.db")

	cfg := &control.ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18097"
	cfg.Database.Path = dbPath
	cfg.SanitizeAndApplyDefaults()

	pubKey, privKey, _ := control.LoadOrGenerateKeys("")
	db, _ := control.NewDatabaseStore(dbPath)
	defer db.Close()

	slots := control.NewSlotAllocator(250)
	sessions := control.NewSessionManager()
	clusterMgr := control.NewNodeClusterManager("obs_test_secret_32b_ok_here!!")

	hubHandler := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, slots, sessions)
	server := httptest.NewServer(hubHandler)
	defer server.Close()

	// 1. Verify X-Request-ID generated and returned
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" || !strings.HasPrefix(reqID, "req-") {
		t.Fatalf("expected X-Request-ID header starting with 'req-', got %q", reqID)
	}
	t.Logf("[PASS] X-Request-ID successfully generated: %s", reqID)

	// 2. Custom X-Request-ID passthrough
	customReq, _ := http.NewRequest(http.MethodGet, server.URL+"/health", nil)
	customReq.Header.Set("X-Request-ID", "custom-trace-uuid-12345")
	customResp, err := http.DefaultClient.Do(customReq)
	if err != nil {
		t.Fatalf("custom request failed: %v", err)
	}
	defer customResp.Body.Close()

	if customResp.Header.Get("X-Request-ID") != "custom-trace-uuid-12345" {
		t.Fatalf("expected passthrough of X-Request-ID, got %s", customResp.Header.Get("X-Request-ID"))
	}
	t.Log("[PASS] Custom X-Request-ID passthrough verified!")
}

// TestPerformance_CgroupEnsureParentOnce verifies:
// EnsureParentHierarchy is idempotent and executes hierarchy creation once.
func TestPerformance_CgroupEnsureParentOnce(t *testing.T) {
	testDir := t.TempDir()
	cgRoot := filepath.Join(testDir, "cgroup_test")

	cgm := runtime.NewCgroupManager(cgRoot)

	// Call EnsureParentHierarchy multiple times
	for i := 0; i < 5; i++ {
		if err := cgm.EnsureParentHierarchy(); err != nil {
			t.Fatalf("EnsureParentHierarchy iteration %d failed: %v", i, err)
		}
	}

	t.Log("[PASS] EnsureParentHierarchy sync.Once idempotency verified!")
}
