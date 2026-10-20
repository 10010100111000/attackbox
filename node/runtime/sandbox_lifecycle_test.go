package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testRuntime(t *testing.T) *RuntimeManager {
	t.Helper()
	root := t.TempDir()
	return NewRuntimeManager(RuntimeConfig{InstancesDir: filepath.Join(root, "instances"), CgroupRoot: filepath.Join(root, "cgroups")})
}

func addTestRecord(m *RuntimeManager, id string) {
	m.sandboxes[id] = &SandboxRecord{ID: id, State: "running", SlotIndex: 1, StartedAt: time.Now(), LastActiveAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), idleLimit: time.Hour}
}

func TestConcurrentStopsCleanResourcesOnce(t *testing.T) {
	m := testRuntime(t)
	addTestRecord(m, "box")
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	m.SetNetworkLifecycle(nil, func(context.Context, int, string) error { calls.Add(1); close(entered); <-release; return nil })
	errs := make(chan error, 2)
	go func() { errs <- m.DestroySandbox(context.Background(), "box") }()
	<-entered
	go func() { errs <- m.DestroySandbox(context.Background(), "box") }()
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("network teardown called %d times", calls.Load())
	}
	if _, exists := m.GetSnapshot("box"); exists {
		t.Fatal("cleaned instance retained")
	}
}

func TestCleanupFailureRetriedBeforeTTL(t *testing.T) {
	m := testRuntime(t)
	addTestRecord(m, "box")
	var calls int
	m.SetNetworkLifecycle(nil, func(context.Context, int, string) error {
		calls++
		if calls == 1 {
			return errors.New("temporary teardown failure")
		}
		return nil
	})
	if err := m.DestroySandbox(context.Background(), "box"); err == nil {
		t.Fatal("cleanup error swallowed")
	}
	s, exists := m.GetSnapshot("box")
	if !exists || s.State != "cleanup_failed" || s.CleanupError == "" {
		t.Fatalf("lost retry state: %+v", s)
	}
	m.ReapSandboxes(context.Background(), time.Now())
	if _, exists := m.GetSnapshot("box"); exists {
		t.Fatal("cleanup was not retried independently of TTL")
	}
}

func TestFailedCreateKeepsOwnershipUntilCleanup(t *testing.T) {
	m := testRuntime(t)
	if err := os.WriteFile(m.cfg.CgroupRoot, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := m.CreateSandbox(context.Background(), SandboxConfig{ID: "partial", UserID: 7, SlotIndex: 3})
	if err == nil {
		t.Fatal("expected create failure")
	}
	s, exists := m.GetSnapshot("partial")
	if !exists || s.UserID != 7 || s.SlotIndex != 3 || s.State != "cleanup_failed" {
		t.Fatalf("lost partial allocation: %+v", s)
	}
	if err := os.Remove(m.cfg.CgroupRoot); err != nil {
		t.Fatal(err)
	}
	if err := m.DestroySandbox(context.Background(), "partial"); err != nil {
		t.Fatal(err)
	}
	if _, exists := m.GetSnapshot("partial"); exists {
		t.Fatal("retry did not remove record")
	}
}

func TestRestartCleansUntrackedSnapshot(t *testing.T) {
	m := testRuntime(t)
	rootfs := m.RootfsPath("residual")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	// A local filesystem stand-in for btrfs; every path is a test temp directory.
	if err := os.WriteFile(filepath.Join(bin, "btrfs"), []byte("#!/bin/sh\n[ \"$1 $2\" = \"subvolume delete\" ] || exit 1\nrmdir \"$3\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := m.CleanupResidualSandboxes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(rootfs)); !os.IsNotExist(err) {
		t.Fatalf("residual snapshot remains: %v", err)
	}
}

func TestSnapshotsRemainSafeDuringActivity(t *testing.T) {
	m := testRuntime(t)
	addTestRecord(m, "box")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			m.TouchSandbox("box")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, inventory := m.Inventory()
			if len(inventory) != 1 {
				t.Error("inconsistent inventory")
			}
			_ = m.ListSnapshots()
		}
	}()
	wg.Wait()
}

func TestSandboxIDCannotEscapeResourceDirectories(t *testing.T) {
	m := testRuntime(t)
	if err := m.DestroySandbox(context.Background(), "../outside"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := m.CreateSandbox(context.Background(), SandboxConfig{ID: "../outside"}); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestRealActivityPreventsIdleReaping(t *testing.T) {
	m := testRuntime(t)
	addTestRecord(m, "box")
	m.sandboxes["box"].LastActiveAt = time.Now().Add(-2 * time.Hour)
	m.TouchSandbox("box")
	m.ReapSandboxes(context.Background(), time.Now())
	if _, exists := m.GetSnapshot("box"); !exists {
		t.Fatal("active terminal was reaped")
	}
	m.mu.Lock()
	m.sandboxes["box"].LastActiveAt = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()
	m.ReapSandboxes(context.Background(), time.Now())
	if _, exists := m.GetSnapshot("box"); exists {
		t.Fatal("idle instance was not reaped")
	}
}
