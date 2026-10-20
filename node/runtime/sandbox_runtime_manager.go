package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func LimitsFromPolicy(q protocol.BoxResourceQuota, rootDeviceNumber ...string) ResourceLimits {
	memMB := q.MemoryMaxMB
	if memMB <= 0 {
		memMB = protocol.DefaultMemLimitMB // Fail-closed: default to 2048MB, never leave unbound
	}
	memBytes := memMB * 1024 * 1024

	var swapBytes int64
	if q.MemorySwapMaxMB > 0 {
		swapBytes = q.MemorySwapMaxMB * 1024 * 1024
	}

	cpuWeight := q.CPUWeight
	if cpuWeight == 0 {
		cpuWeight = protocol.DefaultCPUWeight
	}

	pidsMax := q.PIDsMax
	if pidsMax <= 0 {
		pidsMax = protocol.DefaultPIDsLimit
	}

	ioBytes := q.IOWriteBytesSec
	if ioBytes <= 0 {
		ioBytes = protocol.DefaultIOWriteBytesSec
	}

	iops := q.IOWriteIOPS
	if iops <= 0 {
		iops = protocol.DefaultIOWriteIOPS
	}

	var rootDev string
	if len(rootDeviceNumber) > 0 {
		rootDev = rootDeviceNumber[0]
	}

	return ResourceLimits{
		PIDsMax:          pidsMax,
		MemoryMaxBytes:   memBytes,
		SwapMaxBytes:     swapBytes,
		CPUWeight:        cpuWeight,
		IOWriteBytesSec:  ioBytes,
		IOWriteIOPS:      iops,
		RootDeviceNumber: rootDev,
	}
}

type SandboxConfig struct {
	ID     string
	Policy protocol.SandboxPolicy
}

type SandboxState struct {
	ID           string
	PID          int
	RootfsPath   string
	CgroupPath   string
	Policy       protocol.SandboxPolicy
	StartedAt    time.Time
	LastActiveAt time.Time
	CreatedAt    time.Time
}

// -------------------------------------------------------------
// Runtime Manager
// -------------------------------------------------------------

type RuntimeManager struct {
	cfg              RuntimeConfig
	btrfs            *BtrfsDriver
	cgroups          *CgroupManager
	diskGuard        *DiskGuard
	rootDeviceNumber string
	mu               sync.RWMutex
	sandboxes        map[string]*SandboxState
	commands         map[string]*exec.Cmd
}

func NewRuntimeManager(cfg RuntimeConfig) *RuntimeManager {
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
		sandboxes:        make(map[string]*SandboxState),
		commands:         make(map[string]*exec.Cmd),
	}

	mgr.diskGuard = NewDiskGuard(cfg.DiskGuard, "/", func(freeMB int64) {
		log.Printf("[ALERT] Host disk circuit breaker activated at %d MB free.", freeMB)
	})

	return mgr
}

// DetectDeviceNumber detects the underlying physical block device major:minor (whole disk) for a given directory.
// For filesystems like Btrfs where mountinfo reports anonymous virtual device (0:X), it extracts the real
// backing device path (e.g. /dev/vda1) and traverses sysfs to find the whole disk device number (e.g. 254:0).
func DetectDeviceNumber(path string) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	scanner := bufio.NewScanner(f)
	var bestMountPoint string
	var bestDevSource string
	var fallbackDevNum string

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 5 {
			devNum := parts[2]
			mountPoint := parts[4]

			if strings.HasPrefix(absPath, mountPoint) && len(mountPoint) > len(bestMountPoint) {
				bestMountPoint = mountPoint
				fallbackDevNum = devNum

				// Find optional fields separator "-"
				for i := 6; i < len(parts); i++ {
					if parts[i] == "-" && i+2 < len(parts) {
						bestDevSource = parts[i+2]
						break
					}
				}
			}
		}
	}

	if strings.HasPrefix(bestDevSource, "/dev/") {
		devName := filepath.Base(bestDevSource)
		sysBlockPath := filepath.Join("/sys/class/block", devName)
		realPath, err := filepath.EvalSymlinks(sysBlockPath)
		if err == nil {
			// Check if it's a partition (partitions have a "partition" sysfs attribute file)
			partitionFile := filepath.Join(realPath, "partition")
			if _, err := os.Stat(partitionFile); err == nil {
				// It's a partition; the parent directory is the whole disk device
				parentDevFile := filepath.Join(filepath.Dir(realPath), "dev")
				if data, err := os.ReadFile(parentDevFile); err == nil {
					val := strings.TrimSpace(string(data))
					if val != "" {
						return val
					}
				}
			}

			// Whole disk direct dev
			devFile := filepath.Join(realPath, "dev")
			if data, err := os.ReadFile(devFile); err == nil {
				val := strings.TrimSpace(string(data))
				if val != "" {
					return val
				}
			}
		}
	}

	// Fallback to non-zero mountinfo major device if sysfs not available
	if fallbackDevNum != "" && !strings.HasPrefix(fallbackDevNum, "0:") {
		return fallbackDevNum
	}
	return ""
}

func (m *RuntimeManager) Start(ctx context.Context) error {
	m.diskGuard.Start(ctx)
	if err := m.btrfs.EnsureBaseRootfs(); err != nil {
		return fmt.Errorf("verify golden base image: %w", err)
	}
	m.RecoverAndReconcileLocalSandboxes()
	m.StartLocalReaper(ctx)
	return nil
}

func (m *RuntimeManager) ListSandboxIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, 0, len(m.sandboxes))
	for id := range m.sandboxes {
		res = append(res, id)
	}
	return res
}

// RecoverAndReconcileLocalSandboxes scans instances dir on startup to adopt existing sandbox resources.
func (m *RuntimeManager) RecoverAndReconcileLocalSandboxes() {
	instDir := m.cfg.InstancesDir
	if instDir == "" {
		instDir = "/var/lib/attackbox/instances"
	}
	entries, err := os.ReadDir(instDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sbxID := entry.Name()
		rootfsPath := filepath.Join(instDir, sbxID, "rootfs")
		cgPath := filepath.Join(m.cfg.CgroupRoot, sbxID)

		info, _ := entry.Info()
		started := time.Now()
		if info != nil {
			started = info.ModTime()
		}

		m.mu.Lock()
		m.sandboxes[sbxID] = &SandboxState{
			ID:           sbxID,
			PID:          0,
			RootfsPath:   rootfsPath,
			CgroupPath:   cgPath,
			StartedAt:    started,
			LastActiveAt: started,
			CreatedAt:    started,
		}
		m.mu.Unlock()
		log.Printf("[RECOVERY] Adopted local sandbox %s on worker startup", sbxID)
	}
}

// StartLocalReaper runs a fail-closed hard TTL reaper on Worker (2 hours max lifetime).
func (m *RuntimeManager) StartLocalReaper(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				m.mu.RLock()
				var toReap []string
				for id, sbx := range m.sandboxes {
					if now.Sub(sbx.StartedAt) > 2*time.Hour {
						toReap = append(toReap, id)
					}
				}
				m.mu.RUnlock()

				for _, id := range toReap {
					log.Printf("[LOCAL-REAPER] Sandbox %s exceeded 2h hard TTL ceiling on worker, force destroying...", id)
					_ = m.DestroySandbox(context.Background(), id)
				}
			}
		}
	}()
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

	limits := LimitsFromPolicy(scfg.Policy.Resources, m.rootDeviceNumber)
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

	now := time.Now()
	state := &SandboxState{
		ID:           scfg.ID,
		PID:          cmd.Process.Pid,
		RootfsPath:   rootfsPath,
		CgroupPath:   cgPath,
		Policy:       scfg.Policy,
		StartedAt:    now,
		LastActiveAt: now,
		CreatedAt:    now,
	}

	m.mu.Lock()
	m.sandboxes[scfg.ID] = state
	m.commands[scfg.ID] = cmd
	m.mu.Unlock()

	return state, nil
}

func (m *RuntimeManager) TouchSandbox(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sandboxes[id]; ok {
		s.LastActiveAt = time.Now()
	}
}

func (m *RuntimeManager) DestroySandbox(ctx context.Context, sandboxID string) error {
	m.mu.RLock()
	sbx, exists := m.sandboxes[sandboxID]
	cmd := m.commands[sandboxID]
	m.mu.RUnlock()

	rootfsPath := filepath.Join(m.cfg.InstancesDir, sandboxID, "rootfs")
	if exists && sbx != nil && sbx.RootfsPath != "" {
		rootfsPath = sbx.RootfsPath
	}

	var destroyErrors []string

	// 1. Terminate container process with bounded wait timeout (prevent indefinite hang on D-state)
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()
		select {
		case <-waitCh:
		case <-ctx.Done():
			destroyErrors = append(destroyErrors, fmt.Sprintf("wait process exit: %v", ctx.Err()))
		case <-time.After(5 * time.Second):
			destroyErrors = append(destroyErrors, "wait process exit timed out after 5s")
		}
	}

	// 2. Unmount container filesystems
	if _, err := os.Stat(rootfsPath); err == nil {
		if unmountErr := m.unmountContainerFilesystems(rootfsPath); unmountErr != nil {
			destroyErrors = append(destroyErrors, fmt.Sprintf("unmount rootfs: %v", unmountErr))
		}
	}

	// 3. Destroy cgroups
	if err := m.cgroups.Destroy(sandboxID); err != nil && !os.IsNotExist(err) {
		destroyErrors = append(destroyErrors, fmt.Sprintf("destroy cgroup: %v", err))
	}

	// 4. Destroy Btrfs snapshot
	if err := m.btrfs.DestroySnapshot(sandboxID); err != nil {
		destroyErrors = append(destroyErrors, fmt.Sprintf("destroy btrfs snapshot: %v", err))
	}

	// 5. P1-8 Fix: If any destruction step failed, DO NOT delete state from memory
	// so that subsequent retries still have full context and won't falsely hit a "non-existent" success branch!
	if len(destroyErrors) > 0 {
		return fmt.Errorf("destroy sandbox %s: %s", sandboxID, strings.Join(destroyErrors, "; "))
	}

	// 6. Only purge from memory state once all resources are cleanly wiped
	m.mu.Lock()
	delete(m.sandboxes, sandboxID)
	delete(m.commands, sandboxID)
	m.mu.Unlock()

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

	var errs []string
	for _, mp := range mountPoints {
		if _, err := os.Stat(mp); err == nil {
			out, err := exec.Command("umount", "-l", mp).CombinedOutput()
			if err != nil {
				if isMountPoint(mp) {
					errs = append(errs, fmt.Sprintf("umount %s: %v (%s)", mp, err, strings.TrimSpace(string(out))))
				}
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func isMountPoint(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
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
