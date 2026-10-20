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
	BaseRootfs     string              `yaml:"base_rootfs"`
	InstancesDir   string              `yaml:"instances_dir"`
	CgroupRoot     string              `yaml:"cgroup_root"`
	Limits         RuntimeLimitsConfig `yaml:"limits"`
	DiskGuard      DiskGuardConfig     `yaml:"disk_guard"`
	Tmpfs          TmpfsConfig         `yaml:"tmpfs"`
	SandboxTTLSec  int                 `yaml:"sandbox_ttl_sec"`
	SandboxIdleSec int                 `yaml:"sandbox_idle_sec"`
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
	ID        string
	UserID    int64
	SlotIndex int
	IP        string
	Policy    protocol.SandboxPolicy
}

// SandboxRecord Worker 内部唯一的全量权威实例记录
type SandboxRecord struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	SlotIndex    int       `json:"slot_index"`
	IP           string    `json:"ip"`
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	CleanupError string    `json:"cleanup_error,omitempty"`

	cmd          *exec.Cmd `json:"-"`
}

// SandboxState 兼容别名
type SandboxState = SandboxRecord

// SandboxSnapshot 外部组件只读值快照
type SandboxSnapshot struct {
	ID           string `json:"id"`
	UserID       int64  `json:"user_id"`
	SlotIndex    int    `json:"slot_index"`
	IP           string `json:"ip"`
	PID          int    `json:"pid"`
	StartedAt    int64  `json:"started_at"`
	LastActiveAt int64  `json:"last_active_at"`
	CleanupError string `json:"cleanup_error,omitempty"`
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
	netSetup         func(ctx context.Context, slotIndex int, id, netns string) (string, error)
	netTeardown      func(ctx context.Context, slotIndex int, id string) error
	mu               sync.RWMutex
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

// StartLocalReaper runs authoritative lifecycle reaper on Worker (TTL and Idle timeout).
func (m *RuntimeManager) StartLocalReaper(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				ttlLimit := 2 * time.Hour
				idleLimit := 15 * time.Minute
				if m.cfg.SandboxTTLSec > 0 {
					ttlLimit = time.Duration(m.cfg.SandboxTTLSec) * time.Second
				}
				if m.cfg.SandboxIdleSec > 0 {
					idleLimit = time.Duration(m.cfg.SandboxIdleSec) * time.Second
				}

				m.mu.RLock()
				type reapItem struct {
					id     string
					reason string
				}
				var toReap []reapItem
				for id, sbx := range m.sandboxes {
					if now.Sub(sbx.StartedAt) > ttlLimit {
						toReap = append(toReap, reapItem{id: id, reason: "ttl_expired"})
					} else if idleLimit > 0 && now.Sub(sbx.LastActiveAt) > idleLimit {
						toReap = append(toReap, reapItem{id: id, reason: "idle_timeout"})
					}
				}
				m.mu.RUnlock()

				for _, item := range toReap {
					log.Printf("[LOCAL-REAPER] Sandbox %s reached %s on worker, terminating...", item.id, item.reason)
					_ = m.DestroySandbox(context.Background(), item.id)
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
	cgFile, _, err := m.cgroups.Create(scfg.ID, limits)
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

	sandboxIP := scfg.IP
	if m.netSetup != nil && scfg.SlotIndex > 0 {
		netns := fmt.Sprintf("/proc/%d/ns/net", cmd.Process.Pid)
		ip, netErr := m.netSetup(ctx, scfg.SlotIndex, scfg.ID, netns)
		if netErr != nil {
			_ = cmd.Process.Kill()
			_ = m.btrfs.DestroySnapshot(scfg.ID)
			_ = m.cgroups.Destroy(scfg.ID)
			return nil, fmt.Errorf("setup sandbox network: %w", netErr)
		}
		sandboxIP = ip
	}

	now := time.Now()
	state := &SandboxRecord{
		ID:           scfg.ID,
		UserID:       scfg.UserID,
		SlotIndex:    scfg.SlotIndex,
		IP:           sandboxIP,
		PID:          cmd.Process.Pid,
		StartedAt:    now,
		LastActiveAt: now,
		cmd:          cmd,
	}

	m.mu.Lock()
	m.sandboxes[scfg.ID] = state
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
	if !exists {
		m.mu.RUnlock()
		return nil // 幂等：沙箱已不存在，直接成功
	}
	cmd := sbx.cmd
	m.mu.RUnlock()

	rootfsPath := filepath.Join(m.cfg.InstancesDir, sandboxID, "rootfs")
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

	// 3. Teardown network if configured
	if m.netTeardown != nil && sbx.SlotIndex > 0 {
		if netErr := m.netTeardown(ctx, sbx.SlotIndex, sandboxID); netErr != nil {
			destroyErrors = append(destroyErrors, fmt.Sprintf("teardown network: %v", netErr))
		}
	}

	// 4. Destroy cgroups
	if err := m.cgroups.Destroy(sandboxID); err != nil && !os.IsNotExist(err) {
		destroyErrors = append(destroyErrors, fmt.Sprintf("destroy cgroup: %v", err))
	}

	// 5. Destroy Btrfs snapshot
	if err := m.btrfs.DestroySnapshot(sandboxID); err != nil {
		destroyErrors = append(destroyErrors, fmt.Sprintf("destroy btrfs snapshot: %v", err))
	}

	// 6. If any destruction step failed, record error and retain in memory
	if len(destroyErrors) > 0 {
		m.mu.Lock()
		if cur, ok := m.sandboxes[sandboxID]; ok {
			cur.CleanupError = strings.Join(destroyErrors, "; ")
		}
		m.mu.Unlock()
		return fmt.Errorf("destroy sandbox %s: %s", sandboxID, strings.Join(destroyErrors, "; "))
	}

	// 6. Only purge from memory state once all resources are cleanly wiped
	m.mu.Lock()
	delete(m.sandboxes, sandboxID)
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

func (m *RuntimeManager) GetSnapshot(sandboxID string) (SandboxSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sbx, ok := m.sandboxes[sandboxID]
	if !ok {
		return SandboxSnapshot{}, false
	}
	return SandboxSnapshot{
		ID:           sbx.ID,
		UserID:       sbx.UserID,
		SlotIndex:    sbx.SlotIndex,
		IP:           sbx.IP,
		PID:          sbx.PID,
		StartedAt:    sbx.StartedAt.Unix(),
		LastActiveAt: sbx.LastActiveAt.Unix(),
		CleanupError: sbx.CleanupError,
	}, true
}

func (m *RuntimeManager) ListSnapshots() []SandboxSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]SandboxSnapshot, 0, len(m.sandboxes))
	for _, sbx := range m.sandboxes {
		list = append(list, SandboxSnapshot{
			ID:           sbx.ID,
			UserID:       sbx.UserID,
			SlotIndex:    sbx.SlotIndex,
			IP:           sbx.IP,
			PID:          sbx.PID,
			StartedAt:    sbx.StartedAt.Unix(),
			LastActiveAt: sbx.LastActiveAt.Unix(),
			CleanupError: sbx.CleanupError,
		})
	}
	return list
}

func (m *RuntimeManager) ListSummaries() []protocol.SandboxSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]protocol.SandboxSummary, 0, len(m.sandboxes))
	for _, sbx := range m.sandboxes {
		res = append(res, protocol.SandboxSummary{
			ID:           sbx.ID,
			LastActiveAt: sbx.LastActiveAt.Unix(),
		})
	}
	return res
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
