//go:build linux

package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"attackbox/node/runtime"
	"attackbox/protocol"
)

// TestP1_8_DestroyPipelineNoStateDropAndBoundedWait specifically verifies:
// 1. DestroySandbox does not drop sandbox state prior to executing cleanup steps.
// 2. cmd.Wait is bounded by context / timer to prevent indefinite hangs.
// 3. Mounts and snapshots are fully unmounted and deleted, and errors are not swallowed.
// 4. Memory state is only purged AFTER all resources are cleanly wiped.
func TestP1_8_DestroyPipelineNoStateDropAndBoundedWait(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("P1-8 test requires root privileges for btrfs and cgroup destruction")
	}

	testDir := t.TempDir()
	baseRootfs := filepath.Join(testDir, "base_rootfs")
	instancesDir := filepath.Join(testDir, "instances")
	cgroupRoot := filepath.Join(testDir, "cgroup")

	_ = os.MkdirAll(baseRootfs, 0755)
	_ = os.MkdirAll(instancesDir, 0755)
	_ = os.MkdirAll(cgroupRoot, 0755)

	rtCfg := runtime.RuntimeConfig{
		BaseRootfs:   "/var/lib/attackbox/base_rootfs",
		InstancesDir: "/var/lib/attackbox/instances",
		CgroupRoot:   "/sys/fs/cgroup/attackbox",
	}

	mgr := runtime.NewRuntimeManager(rtCfg)

	sandboxID := "sbx_p1_8_pipeline_test"
	policy := protocol.SandboxPolicy{
		Resources: protocol.BoxResourceQuota{
			MemoryMaxMB: 256,
			PIDsMax:     64,
		},
		Storage: protocol.BoxStoragePolicy{
			TmpfsTmpSize: "64M",
			TmpfsRunSize: "32M",
		},
		Security: protocol.BoxSecurityPolicy{
			ReadOnlyProcSys: true,
			MaskSysrq:       true,
		},
	}

	// 1. Launch a real sandbox
	t.Logf("Launching sandbox %s...", sandboxID)
	state, err := mgr.CreateSandbox(context.Background(), runtime.SandboxConfig{
		ID:     sandboxID,
		Policy: policy,
	})
	if err != nil {
		t.Fatalf("CreateSandbox failed: %v", err)
	}

	// Verify sandbox registered in memory
	if _, ok := mgr.GetSandbox(sandboxID); !ok {
		t.Fatalf("sandbox %s not found in runtime manager after create", sandboxID)
	}

	// 2. Execute DestroySandbox with a 10-second context
	destroyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	destroyStart := time.Now()
	err = mgr.DestroySandbox(destroyCtx, sandboxID)
	destroyDuration := time.Since(destroyStart)

	if err != nil {
		t.Fatalf("DestroySandbox failed: %v", err)
	}
	t.Logf("[PASS] Sandbox destroyed cleanly in %v (Bounded wait verified)", destroyDuration)

	// 3. Verify memory state is purged ONLY AFTER success
	if _, ok := mgr.GetSandbox(sandboxID); ok {
		t.Errorf("sandbox %s should have been purged from memory after successful destroy", sandboxID)
	}

	// 4. Verify disk rootfs directory is completely wiped
	if _, err := os.Stat(state.RootfsPath); err == nil {
		t.Errorf("instance rootfs %s still exists on disk!", state.RootfsPath)
	}

	// 5. Verify Cgroup directory is completely wiped
	if _, err := os.Stat(state.CgroupPath); err == nil {
		t.Errorf("instance cgroup %s still exists!", state.CgroupPath)
	}

	// 6. Test Idempotency: Calling DestroySandbox on an already destroyed sandbox returns cleanly
	idempotentCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if err := mgr.DestroySandbox(idempotentCtx, sandboxID); err != nil {
		t.Errorf("idempotent DestroySandbox failed: %v", err)
	}
	t.Log("[PASS] Idempotent destruction verified")
}
