package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"attackbox/protocol"
)

var (
	ErrDiskFuseTripped = errors.New("sandbox creation rejected: host disk free space is below safety threshold")
	ErrSandboxExists   = errors.New("sandbox with this ID already exists")
	ErrSandboxNotFound = errors.New("sandbox not found")
	ErrInitTimeout     = errors.New("container PID 1 initialization timed out")
)

// -------------------------------------------------------------
// Configuration & Types
// -------------------------------------------------------------

type RuntimeLimitsConfig struct {
	PIDsMax          int64  `yaml:"pids_max"`
	MemoryMaxMB      int64  `yaml:"memory_max_mb"`
	MemorySwapMaxMB  int64  `yaml:"memory_swap_max_mb"`
	CPUWeight        uint64 `yaml:"cpu_weight"`
	IOWriteBytesSec  int64  `yaml:"io_write_bytes_sec"`
	IOWriteIOPS      int64  `yaml:"io_write_iops"`
	RootDeviceNumber string `yaml:"root_device_number"`
}

type TmpfsConfig struct {
	TmpSize string `yaml:"tmp_size"`
	RunSize string `yaml:"run_size"`
}

type DiskGuardConfig struct {
	Enabled          bool  `yaml:"enabled"`
	CheckIntervalSec int   `yaml:"check_interval_sec"`
	MinFreeSpaceMB   int64 `yaml:"min_free_space_mb"`
	WarnFreeSpaceMB  int64 `yaml:"warn_free_space_mb"`
}

type RuntimeConfig struct {
	BaseRootfs   string              `yaml:"base_rootfs"`
	InstancesDir string              `yaml:"instances_dir"`
	CgroupRoot   string              `yaml:"cgroup_root"`
	Limits       RuntimeLimitsConfig `yaml:"limits"`
	DiskGuard    DiskGuardConfig     `yaml:"disk_guard"`
	Tmpfs        TmpfsConfig         `yaml:"tmpfs"`
}

type ResourceLimits struct {
	PIDsMax          int64
	MemoryMaxBytes   int64
	SwapMaxBytes     int64
	CPUWeight        uint64
	IOWriteBytesSec  int64
	IOWriteIOPS      int64
	RootDeviceNumber string
}

func LimitsFromPolicy(q protocol.BoxResourceQuota) ResourceLimits {
	var memBytes int64
	if q.MemoryMaxMB > 0 {
		memBytes = q.MemoryMaxMB * 1024 * 1024
	}
	var swapBytes int64
	if q.MemorySwapMaxMB > 0 {
		swapBytes = q.MemorySwapMaxMB * 1024 * 1024
	}
	cpuWeight := q.CPUWeight
	if cpuWeight == 0 {
		cpuWeight = 100
	}
	pidsMax := q.PIDsMax
	if pidsMax == 0 {
		pidsMax = 256
	}

	return ResourceLimits{
		PIDsMax:         pidsMax,
		MemoryMaxBytes:  memBytes,
		SwapMaxBytes:    swapBytes,
		CPUWeight:       cpuWeight,
		IOWriteBytesSec: q.IOWriteBytesSec,
		IOWriteIOPS:     q.IOWriteIOPS,
	}
}

func LimitsFromConfig(c RuntimeLimitsConfig) ResourceLimits {
	return ResourceLimits{
		PIDsMax:          c.PIDsMax,
		MemoryMaxBytes:   c.MemoryMaxMB * 1024 * 1024,
		SwapMaxBytes:     c.MemorySwapMaxMB * 1024 * 1024,
		CPUWeight:        c.CPUWeight,
		IOWriteBytesSec:  c.IOWriteBytesSec,
		IOWriteIOPS:      c.IOWriteIOPS,
		RootDeviceNumber: c.RootDeviceNumber,
	}
}

type SandboxConfig struct {
	ID     string
	Policy protocol.SandboxPolicy
}

type SandboxState struct {
	ID         string
	PID        int
	RootfsPath string
	CgroupPath string
	Policy     protocol.SandboxPolicy
	StartedAt  time.Time
	CreatedAt  time.Time
}

// -------------------------------------------------------------
// Runtime Manager
// -------------------------------------------------------------

type RuntimeManager struct {
	cfg       RuntimeConfig
	btrfs     *BtrfsDriver
	cgroups   *CgroupManager
	diskGuard *DiskGuard
	mu        sync.RWMutex
	sandboxes map[string]*SandboxState
	commands  map[string]*exec.Cmd
}

func NewRuntimeManager(cfg RuntimeConfig) *RuntimeManager {
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

func (m *RuntimeManager) Start(ctx context.Context) error {
	m.diskGuard.Start(ctx)
	if err := m.btrfs.EnsureBaseRootfs(); err != nil {
		return fmt.Errorf("verify golden base image: %w", err)
	}
	return nil
}

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

func (m *RuntimeManager) CreateSandbox(ctx context.Context, scfg SandboxConfig) (*SandboxState, error) {
	if m.diskGuard.IsTripped() {
		return nil, ErrDiskFuseTripped
	}

	m.mu.Lock()
	if _, exists := m.sandboxes[scfg.ID]; exists {
		m.mu.Unlock()
		return nil, ErrSandboxExists
	}
	m.mu.Unlock()

	limits := LimitsFromPolicy(scfg.Policy.Resources)
	cgFile, cgPath, err := m.cgroups.Create(scfg.ID, limits)
	if err != nil {
		return nil, fmt.Errorf("create cgroup: %w", err)
	}
	defer cgFile.Close()

	rootfsPath, err := m.btrfs.CreateSnapshot(scfg.ID)
	if err != nil {
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("create btrfs snapshot: %w", err)
	}

	targetWrapper := filepath.Join(rootfsPath, "usr/bin/attackbox-enter")
	if _, err := os.Stat(targetWrapper); err != nil {
		if hostData, err := os.ReadFile("/usr/local/bin/attackbox-enter"); err == nil {
			_ = os.WriteFile(targetWrapper, hostData, 0755)
		}
	}

	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("create sync pipe: %w", err)
	}
	defer rPipe.Close()

	exePath, err := os.Executable()
	if err != nil {
		exePath = "/proc/self/exe"
	}

	policyBytes, err := json.Marshal(scfg.Policy)
	if err != nil {
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("marshal policy json: %w", err)
	}

	cmd := exec.Command(exePath, "__init__", scfg.ID, rootfsPath)
	cmd.ExtraFiles = []*os.File{wPipe}
	cmd.Env = append(os.Environ(), "ATTACKBOX_POLICY="+string(policyBytes))
	configureSysProcAttr(cmd, cgFile)

	if err := cmd.Start(); err != nil {
		_ = wPipe.Close()
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, fmt.Errorf("start container init process: %w", err)
	}
	_ = wPipe.Close()

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
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		_ = m.btrfs.DestroySnapshot(scfg.ID)
		_ = m.cgroups.Destroy(scfg.ID)
		return nil, ErrInitTimeout
	case err := <-readyChan:
		if err != nil {
			_ = cmd.Process.Kill()
			_ = m.btrfs.DestroySnapshot(scfg.ID)
			_ = m.cgroups.Destroy(scfg.ID)
			return nil, fmt.Errorf("container init failed: %w", err)
		}
	}

	state := &SandboxState{
		ID:         scfg.ID,
		PID:        cmd.Process.Pid,
		RootfsPath: rootfsPath,
		CgroupPath: cgPath,
		Policy:     scfg.Policy,
		StartedAt:  time.Now(),
		CreatedAt:  time.Now(),
	}

	m.mu.Lock()
	m.sandboxes[scfg.ID] = state
	m.commands[scfg.ID] = cmd
	m.mu.Unlock()

	return state, nil
}

func (m *RuntimeManager) DestroySandbox(ctx context.Context, sandboxID string) error {
	m.mu.Lock()
	sbx, exists := m.sandboxes[sandboxID]
	cmd := m.commands[sandboxID]
	delete(m.sandboxes, sandboxID)
	delete(m.commands, sandboxID)
	m.mu.Unlock()

	if !exists && sbx == nil {
		cgPath := filepath.Join(m.cfg.CgroupRoot, sandboxID)
		if _, err := os.Stat(cgPath); err == nil {
			_ = m.cgroups.Destroy(sandboxID)
		}
		_ = m.btrfs.DestroySnapshot(sandboxID)
		return nil
	}

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	_ = m.cgroups.Destroy(sandboxID)

	if cmd != nil {
		_ = cmd.Wait()
	}

	_ = m.unmountContainerFilesystems(sbx.RootfsPath)

	if err := m.btrfs.DestroySnapshot(sandboxID); err != nil {
		return fmt.Errorf("destroy btrfs snapshot: %w", err)
	}

	return nil
}

func (m *RuntimeManager) unmountContainerFilesystems(rootfsPath string) error {
	mountPoints := []string{
		filepath.Join(rootfsPath, "dev/shm"),
		filepath.Join(rootfsPath, "dev/pts"),
		filepath.Join(rootfsPath, "dev"),
		filepath.Join(rootfsPath, "proc/sys"),
		filepath.Join(rootfsPath, "proc"),
		filepath.Join(rootfsPath, "sys"),
		filepath.Join(rootfsPath, "run"),
		filepath.Join(rootfsPath, "tmp"),
	}

	for _, mp := range mountPoints {
		if _, err := os.Stat(mp); err == nil {
			_ = exec.Command("umount", "-l", mp).Run()
		}
	}
	return nil
}

func (m *RuntimeManager) GetSandbox(sandboxID string) (*SandboxState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sbx, ok := m.sandboxes[sandboxID]
	return sbx, ok
}

func (m *RuntimeManager) ListSandboxes() []*SandboxState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*SandboxState, 0, len(m.sandboxes))
	for _, sbx := range m.sandboxes {
		list = append(list, sbx)
	}
	return list
}

func (m *RuntimeManager) DiskGuard() *DiskGuard {
	return m.diskGuard
}
