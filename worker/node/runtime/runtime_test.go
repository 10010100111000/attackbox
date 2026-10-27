package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"attackbox/node/runtime"
	"attackbox/protocol"
)

func TestMain(m *testing.M) {
	// If this test binary is re-executed as container init, dispatch and exit immediately
	if runtime.InitDispatcher() {
		return
	}
	os.Exit(m.Run())
}

func TestBtrfsAndCgroupSandboxLifecycle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test: requires root privileges (btrfs and cgroup operations)")
	}

	cfg := runtime.RuntimeConfig{
		BaseRootfs:   "/var/lib/attackbox/base_rootfs",
		InstancesDir: "/var/lib/attackbox/instances",
		CgroupRoot:   "/sys/fs/cgroup/attackbox",
		DiskGuard: runtime.DiskGuardConfig{
			Enabled:        true,
			MinFreeSpaceMB: 5120,
		},
	}

	mgr := runtime.NewRuntimeManager(cfg)
	ctx := context.Background()

	testID := "test_sbx_lifecycle_01"

	// Ensure any dirty previous test is cleaned up first
	_ = mgr.DestroySandbox(ctx, testID)

	// 1. Create sandbox
	t.Logf("Creating sandbox %s with btrfs snapshot and Cgroups v2...", testID)
	start := time.Now()
	sbx, err := mgr.CreateSandbox(ctx, runtime.SandboxConfig{
		ID: testID,
		Policy: protocol.SandboxPolicy{
			Resources: protocol.BoxResourceQuota{
				PIDsMax:         32,
				MemoryMaxMB:     128,
				MemorySwapMaxMB: 0,
				CPUWeight:       100,
				IOWriteBytesSec: 10 * 1024 * 1024,
				IOWriteIOPS:     200,
			},
			Storage: protocol.BoxStoragePolicy{
				TmpfsTmpSize: "64M",
				TmpfsRunSize: "32M",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSandbox failed: %v", err)
	}
	elapsed := time.Since(start)
	rootfsPath := mgr.RootfsPath(testID)
	t.Logf("Sandbox created in %v, PID: %d, Rootfs: %s", elapsed, sbx.PID, rootfsPath)

	// 2. Validate PID is running
	if sbx.PID <= 0 {
		t.Fatalf("expected valid PID, got %d", sbx.PID)
	}

	// 3. Verify Cgroup directory & limits
	cgPath := filepath.Join(cfg.CgroupRoot, testID)
	pidsMaxData, err := os.ReadFile(filepath.Join(cgPath, "pids.max"))
	if err != nil {
		t.Fatalf("read pids.max: %v", err)
	}
	if strings.TrimSpace(string(pidsMaxData)) != "32" {
		t.Fatalf("expected pids.max=32, got %s", strings.TrimSpace(string(pidsMaxData)))
	}

	// 4. Verify PID is inside cgroup.procs
	procsData, err := os.ReadFile(filepath.Join(cgPath, "cgroup.procs"))
	if err != nil {
		t.Fatalf("read cgroup.procs: %v", err)
	}
	if !strings.Contains(string(procsData), strconv.Itoa(sbx.PID)) {
		t.Fatalf("expected PID %d inside cgroup.procs, got: %q", sbx.PID, string(procsData))
	}

	// 5. Verify btrfs rootfs exists
	if _, err := os.Stat(rootfsPath); err != nil {
		t.Fatalf("rootfs directory does not exist: %v", err)
	}

	// 6. Test Destruction Pipeline (kill -> unmount -> btrfs delete -> cgroup delete)
	t.Logf("Destroying sandbox %s...", testID)
	destroyStart := time.Now()
	if err := mgr.DestroySandbox(ctx, testID); err != nil {
		t.Fatalf("DestroySandbox failed: %v", err)
	}
	destroyElapsed := time.Since(destroyStart)
	t.Logf("Sandbox destroyed cleanly in %v (Zero hang verified!)", destroyElapsed)

	// 7. Verify cgroup directory is removed
	if _, err := os.Stat(cgPath); !os.IsNotExist(err) {
		t.Fatalf("expected cgroup dir %s to be deleted, but still exists", cgPath)
	}

	// 8. Verify rootfs directory is removed
	if _, err := os.Stat(rootfsPath); !os.IsNotExist(err) {
		t.Fatalf("expected rootfs dir %s to be deleted, but still exists", rootfsPath)
	}
}
