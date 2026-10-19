package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"attackbox/internal/worker/config"
)

var (
	ErrDiskFuseTripped   = errors.New("sandbox creation rejected: host disk free space is below safety threshold")
	ErrSandboxExists     = errors.New("sandbox with this ID already exists")
	ErrSandboxNotFound   = errors.New("sandbox not found")
	ErrInitTimeout       = errors.New("container PID 1 initialization timed out")
)

// RuntimeManager coordinates btrfs snapshots, cgroups v2, and container lifecycle.
type RuntimeManager struct {
	cfg         config.RuntimeConfig
	btrfs       *BtrfsDriver
	cgroups     *CgroupManager
	diskGuard   *DiskGuard
	mu          sync.RWMutex
	sandboxes   map[string]*SandboxState
	commands    map[string]*exec.Cmd
}

// NewRuntimeManager constructs the container runtime manager.
func NewRuntimeManager(cfg config.RuntimeConfig) *RuntimeManager {
	mgr := &RuntimeManager{
		cfg:       cfg,
		btrfs:     NewBtrfsDriver(cfg.BaseRootfs, cfg.InstancesDir),
		cgroups:   NewCgroupManager(cfg.CgroupRoot),
		sandboxes: make(map[string]*SandboxState),
		commands:  make(map[string]*exec.Cmd),
	}

	mgr.diskGuard = NewDiskGuard(cfg.DiskGuard, "/", func(freeMB int64) {
		fmt.Printf("[ALERT] Host disk circuit breaker activated at %d MB free.\n", freeMB)
	})

	return mgr
}

// Start begins background monitors (disk guard, base image validation).
func (m *RuntimeManager) Start(ctx context.Context) error {
	m.diskGuard.Start(ctx)
	return nil
}

// Stop cleanly terminates all running sandboxes and background monitors.
func (m *RuntimeManager) Stop() {
	m.diskGuard.Stop()

	m.mu.Lock()
	ids := make([]string, 0, len(m.sandboxes))
	for id := range m.sandboxes {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		_ = m.DestroySandbox(context.Background(), id)
	}
}

// CreateSandbox provisions a new isolated ephemeral container instance.
func (m *RuntimeManager) CreateSandbox(ctx context.Context, scfg SandboxConfig) (*SandboxState, error) {
	// 1. Check disk watermark guard
	if m.diskGuard.IsTripped() {
		return nil, ErrDiskFuseTripped
	}

	m.mu.Lock()
	if _, exists := m.sandboxes[scfg.ID]; exists {
		m.mu.Unlock()
		return nil, ErrSandboxExists
	}
	m.mu.Unlock()

	// 2. Prepare Cgroups v2 directory & limits
	cgFile, cgPath, err := m.cgroups.Create(scfg.ID, scfg.Limits)
	if err != nil {
		return nil, fmt.Errorf("create cgroup: %w", err)
	}
	defer cgFile.Close()

	// 3. Create writable transient btrfs subvolume snapshot
	rootfsPath, err := m.btrfs.CreateSnapshot(scfg.ID)
	if err != nil {
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("create btrfs snapshot: %w", err)
	}

	// 4. Setup synchronization pipe
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("create sync pipe: %w", err)
	}
	defer rPipe.Close()

	// 5. Spawn single-binary re-exec PID 1 container entrypoint
	exePath, err := os.Executable()
	if err != nil {
		exePath = "/proc/self/exe"
	}

	tmpSize := scfg.TmpfsTmpSize
	if tmpSize == "" {
		tmpSize = m.cfg.Tmpfs.TmpSize
	}
	runSize := scfg.TmpfsRunSize
	if runSize == "" {
		runSize = m.cfg.Tmpfs.RunSize
	}

	cmd := exec.Command(exePath, "__init__", scfg.ID, rootfsPath, tmpSize, runSize)
	cmd.ExtraFiles = []*os.File{wPipe}
	configureSysProcAttr(cmd, cgFile)

	if err := cmd.Start(); err != nil {
		_ = wPipe.Close()
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("start container init process: %w", err)
	}
	// Close parent's copy of wPipe so EOF is received if child crashes
	_ = wPipe.Close()

	// 6. Wait for READY acknowledgment with timeout
	readyChan := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(rPipe)
		line, err := reader.ReadString('\n')
		if err != nil {
			readyChan <- fmt.Errorf("read init ready signal: %w", err)
			return
		}
		if line != "READY\n" {
			readyChan <- fmt.Errorf("unexpected ready signal: %q", line)
			return
		}
		readyChan <- nil
	}()

	select {
	case err := <-readyChan:
		if err != nil {
			_ = m.cgroups.Kill(scfg.ID)
			_ = cmd.Wait()
			_ = m.cgroups.Destroy(scfg.ID)
			_ = m.btrfs.DestroySnapshot(scfg.ID)
			return nil, fmt.Errorf("container init handshake failed: %w", err)
		}
	case <-time.After(5 * time.Second):
		_ = m.cgroups.Kill(scfg.ID)
		_ = cmd.Wait()
		_ = m.cgroups.Destroy(scfg.ID)
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		return nil, ErrInitTimeout
	case <-ctx.Done():
		_ = m.cgroups.Kill(scfg.ID)
		_ = cmd.Wait()
		_ = m.cgroups.Destroy(scfg.ID)
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		return nil, ctx.Err()
	}

	// 7. Register state
	state := &SandboxState{
		ID:         scfg.ID,
		PID:        cmd.Process.Pid,
		RootfsPath: rootfsPath,
		CgroupPath: cgPath,
		Limits:     scfg.Limits,
		StartedAt:  time.Now(),
	}

	m.mu.Lock()
	m.sandboxes[scfg.ID] = state
	m.commands[scfg.ID] = cmd
	m.mu.Unlock()

	return state, nil
}

// DestroySandbox executes the strict reverse teardown pipeline.
func (m *RuntimeManager) DestroySandbox(ctx context.Context, sandboxID string) error {
	m.mu.Lock()
	_, exists := m.sandboxes[sandboxID]
	cmd, hasCmd := m.commands[sandboxID]
	if !exists {
		m.mu.Unlock()
		return ErrSandboxNotFound
	}
	delete(m.sandboxes, sandboxID)
	delete(m.commands, sandboxID)
	m.mu.Unlock()

	// Step 1: Nuclear kill of all processes inside cgroup
	_ = m.cgroups.Kill(sandboxID)

	// Step 2: Reap child process
	if hasCmd && cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			_ = cmd.Process.Kill()
		}
	}

	// Step 3: Remove Cgroup directory
	_ = m.cgroups.Destroy(sandboxID)

	// Step 4: Atomic subvolume delete on pure unmounted/unreferenced rootfs
	if err := m.btrfs.DestroySnapshot(sandboxID); err != nil {
		return fmt.Errorf("destroy btrfs snapshot: %w", err)
	}

	return nil
}

// GetSandbox returns the state of an active sandbox.
func (m *RuntimeManager) GetSandbox(sandboxID string) (*SandboxState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sbx, ok := m.sandboxes[sandboxID]
	return sbx, ok
}

// ListSandboxes returns a snapshot slice of all active sandboxes.
func (m *RuntimeManager) ListSandboxes() []*SandboxState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*SandboxState, 0, len(m.sandboxes))
	for _, sbx := range m.sandboxes {
		list = append(list, sbx)
	}
	return list
}

// DiskGuard returns the internal host disk watermark monitor.
func (m *RuntimeManager) DiskGuard() *DiskGuard {
	return m.diskGuard
}
