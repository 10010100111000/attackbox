package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"
)

var (
	ErrDiskFuseTripped = errors.New("sandbox creation rejected: host disk free space is below safety threshold")
	ErrSandboxExists   = errors.New("sandbox with this ID already exists")
	ErrSandboxNotFound = errors.New("sandbox not found")
	ErrInitTimeout     = errors.New("container PID 1 initialization timed out")
)

type RuntimeManager struct {
	cfg              RuntimeConfig
	btrfs            *BtrfsDriver
	cgroups          *CgroupManager
	diskGuard        *DiskGuard
	rootDeviceNumber string
	netSetup         func(ctx context.Context, slotIndex int, id, netns string) (string, error)
	netTeardown      func(ctx context.Context, slotIndex int, id string) error
	mu               sync.RWMutex
	revision         uint64
	reaperCancel     context.CancelFunc
	reaperWG         sync.WaitGroup
	guiMu            sync.Mutex
	sandboxes        map[string]*SandboxRecord
}

// SetNetworkLifecycle attaches network provisioning and teardown handlers to the runtime manager.
func (m *RuntimeManager) SetNetworkLifecycle(
	setup func(ctx context.Context, slotIndex int, id, netns string) (string, error),
	teardown func(ctx context.Context, slotIndex int, id string) error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.netSetup = setup
	m.netTeardown = teardown
}

func NewRuntimeManager(cfg RuntimeConfig) *RuntimeManager {
	if cfg.BaseRootfs == "" {
		cfg.BaseRootfs = "/var/lib/attackbox/base_rootfs"
	}
	if cfg.InstancesDir == "" {
		cfg.InstancesDir = "/var/lib/attackbox/instances"
	}
	if cfg.CgroupRoot == "" {
		cfg.CgroupRoot = "/sys/fs/cgroup/attackbox"
	}
	if cfg.SandboxTTLSec <= 0 {
		cfg.SandboxTTLSec = 7200
	}
	if cfg.SandboxIdleSec <= 0 {
		cfg.SandboxIdleSec = 900
	}

	rootDev := cfg.Limits.RootDeviceNumber
	if rootDev == "" {
		instDir := cfg.InstancesDir
		if instDir == "" {
			instDir = "/var/lib/attackbox/instances"
		}
		rootDev = DetectDeviceNumber(instDir)
	}

	mgr := &RuntimeManager{
		cfg:              cfg,
		btrfs:            NewBtrfsDriver(cfg.BaseRootfs, cfg.InstancesDir),
		cgroups:          NewCgroupManager(cfg.CgroupRoot),
		rootDeviceNumber: rootDev,
		sandboxes:        make(map[string]*SandboxRecord),
	}

	mgr.diskGuard = NewDiskGuard(cfg.DiskGuard, "/", func(freeMB int64) {
		log.Printf("[ALERT] Host disk circuit breaker activated at %d MB free.", freeMB)
	})

	return mgr
}

func (m *RuntimeManager) Start(ctx context.Context) error {
	if err := m.btrfs.EnsureBaseRootfs(); err != nil {
		return fmt.Errorf("verify golden base image: %w", err)
	}
	if err := m.CleanupResidualSandboxes(ctx); err != nil {
		return err
	}
	m.diskGuard.Start(ctx)
	m.StartLocalReaper(ctx)
	return nil
}

func (m *RuntimeManager) Stop() {
	if m.reaperCancel != nil {
		m.reaperCancel()
	}
	m.reaperWG.Wait()
	m.diskGuard.Stop()
	for _, sbx := range m.ListSnapshots() {
		_ = m.DestroySandbox(context.Background(), sbx.ID)
	}
}

func (m *RuntimeManager) DiskGuard() *DiskGuard {
	return m.diskGuard
}

func (m *RuntimeManager) CgroupPath(sandboxID string) string {
	if m.cfg.CgroupRoot == "" {
		return filepath.Join("/sys/fs/cgroup/attackbox", sandboxID)
	}
	return filepath.Join(m.cfg.CgroupRoot, sandboxID)
}

func (m *RuntimeManager) RootfsPath(sandboxID string) string {
	instDir := m.cfg.InstancesDir
	if instDir == "" {
		instDir = "/var/lib/attackbox/instances"
	}
	return filepath.Join(instDir, sandboxID, "rootfs")
}
