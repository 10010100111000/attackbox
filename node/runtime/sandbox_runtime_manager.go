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
	"regexp"
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
	TTLSeconds  int
	IdleSeconds int
	ID          string
	UserID      int64
	SlotIndex   int
	IP          string
	Policy      protocol.SandboxPolicy
}

// SandboxRecord Worker 内部唯一的全量权威实例记录
type SandboxRecord struct {
	opMu         sync.Mutex
	State        string    `json:"state"`
	Revision     uint64    `json:"revision"`
	ExpiresAt    time.Time `json:"expires_at"`
	waitDone     chan struct{}
	idleLimit    time.Duration
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	SlotIndex    int       `json:"slot_index"`
	IP           string    `json:"ip"`
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	CleanupError string    `json:"cleanup_error,omitempty"`

	cmd *exec.Cmd `json:"-"`
}

// SandboxSnapshot 外部组件只读值快照
type SandboxSnapshot struct {
	IdleSeconds  int64  `json:"idle_seconds"`
	State        string `json:"state"`
	Revision     uint64 `json:"revision"`
	ExpiresAt    int64  `json:"expires_at"`
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
	revision         uint64
	reaperCancel     context.CancelFunc
	reaperWG         sync.WaitGroup
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
// ReapSandboxes uses the same serialized stop path as explicit requests.
func (m *RuntimeManager) ReapSandboxes(ctx context.Context, now time.Time) {
	for _, sbx := range m.ListSnapshots() {
		if sbx.State == "creating" || sbx.State == "stopping" {
			continue
		}
		if sbx.CleanupError != "" || now.Unix() >= sbx.ExpiresAt || now.Sub(time.Unix(sbx.LastActiveAt, 0)) >= time.Duration(sbx.IdleSeconds)*time.Second {
			_ = m.DestroySandbox(ctx, sbx.ID)
		}
	}
}

func (m *RuntimeManager) StartLocalReaper(ctx context.Context) {
	ctx, m.reaperCancel = context.WithCancel(ctx)
	m.reaperWG.Add(1)
	go func() {
		defer m.reaperWG.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.ReapSandboxes(ctx, now)
			}
		}
	}()
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

var sandboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// CleanupResidualSandboxes implements restart-as-reset. No new work is accepted
// until all leftovers have been cleaned, including resources without a rootfs.
func (m *RuntimeManager) CleanupResidualSandboxes(ctx context.Context) error {
	ids := make(map[string]bool)
	for _, dir := range []string{m.cfg.InstancesDir, m.cfg.CgroupRoot} {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("scan residual resources: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				ids[e.Name()] = true
			}
		}
	}
	var errs []error
	for id := range ids {
		if !sandboxIDPattern.MatchString(id) {
			errs = append(errs, fmt.Errorf("invalid residual sandbox ID %q", id))
			continue
		}
		if err := m.DestroySandbox(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *RuntimeManager) CreateSandbox(ctx context.Context, scfg SandboxConfig) (result SandboxSnapshot, err error) {
	if !sandboxIDPattern.MatchString(scfg.ID) {
		return result, errors.New("invalid sandbox ID")
	}
	if m.diskGuard.IsTripped() {
		return result, ErrDiskFuseTripped
	}
	rec := &SandboxRecord{ID: scfg.ID, UserID: scfg.UserID, SlotIndex: scfg.SlotIndex, State: "creating", StartedAt: time.Now(), LastActiveAt: time.Now()}
	ttl := scfg.TTLSeconds
	if ttl <= 0 {
		ttl = m.cfg.SandboxTTLSec
	}
	idle := scfg.IdleSeconds
	if idle <= 0 {
		idle = m.cfg.SandboxIdleSec
	}
	rec.ExpiresAt = rec.StartedAt.Add(time.Duration(ttl) * time.Second)
	rec.idleLimit = time.Duration(idle) * time.Second
	rec.opMu.Lock()
	defer rec.opMu.Unlock()
	m.mu.Lock()
	if _, exists := m.sandboxes[scfg.ID]; exists {
		m.mu.Unlock()
		return result, ErrSandboxExists
	}
	m.revision++
	rec.Revision = m.revision
	m.sandboxes[scfg.ID] = rec
	m.mu.Unlock()
	defer func() {
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err = errors.Join(err, m.destroyRecord(cleanupCtx, rec))
		}
	}()
	limits := LimitsFromPolicy(scfg.Policy.Resources, m.rootDeviceNumber)
	cgFile, _, err := m.cgroups.Create(scfg.ID, limits)
	if err != nil {
		return SandboxSnapshot{}, fmt.Errorf("create cgroup: %w", err)
	}
	defer cgFile.Close()

	rootfsPath, err := m.btrfs.CreateSnapshot(ctx, scfg.ID)
	if err != nil {

		return SandboxSnapshot{}, fmt.Errorf("create btrfs snapshot: %w", err)
	}

	rPipe, wPipe, err := os.Pipe()
	if err != nil {

		return SandboxSnapshot{}, fmt.Errorf("create sync pipe: %w", err)
	}
	defer rPipe.Close()
	defer wPipe.Close()

	exePath, err := os.Executable()
	if err != nil {
		exePath = "/proc/self/exe"
	}

	policyBytes, err := json.Marshal(scfg.Policy)
	if err != nil {

		return SandboxSnapshot{}, fmt.Errorf("marshal policy json: %w", err)
	}

	cmd := exec.Command(exePath, "__init__", scfg.ID, rootfsPath)
	cmd.ExtraFiles = []*os.File{wPipe}
	cmd.Env = append(os.Environ(), "ATTACKBOX_POLICY="+string(policyBytes))
	configureSysProcAttr(cmd, cgFile)

	if err := cmd.Start(); err != nil {
		_ = wPipe.Close()

		return SandboxSnapshot{}, fmt.Errorf("start container init process: %w", err)
	}
	_ = wPipe.Close()
	m.mu.Lock()
	rec.cmd = cmd
	rec.PID = cmd.Process.Pid
	rec.waitDone = make(chan struct{})
	m.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(rec.waitDone)
		rec.opMu.Lock()
		defer rec.opMu.Unlock()
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.sandboxes[rec.ID] == rec && rec.State == "running" {
			rec.State = "cleanup_failed"
			rec.CleanupError = "container init exited"
			m.revision++
			rec.Revision = m.revision
		}
	}()

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

		return SandboxSnapshot{}, ctx.Err()
	case <-time.After(10 * time.Second):

		return SandboxSnapshot{}, ErrInitTimeout
	case err := <-readyChan:
		if err != nil {

			return SandboxSnapshot{}, fmt.Errorf("container init failed: %w", err)
		}
	}

	sandboxIP := scfg.IP
	if m.netSetup != nil && scfg.SlotIndex > 0 {
		netns := fmt.Sprintf("/proc/%d/ns/net", cmd.Process.Pid)
		ip, netErr := m.netSetup(ctx, scfg.SlotIndex, scfg.ID, netns)
		if netErr != nil {

			return SandboxSnapshot{}, fmt.Errorf("setup sandbox network: %w", netErr)
		}
		sandboxIP = ip
	}

	m.mu.Lock()
	rec.IP = sandboxIP
	rec.State = "running"
	m.revision++
	rec.Revision = m.revision
	result = snapshotOf(rec)
	m.mu.Unlock()
	return result, nil
}

func (m *RuntimeManager) TouchSandbox(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sandboxes[id]; ok {
		s.LastActiveAt = time.Now()
	}
}

func (m *RuntimeManager) DestroySandbox(ctx context.Context, sandboxID string) error {
	if !sandboxIDPattern.MatchString(sandboxID) {
		return errors.New("invalid sandbox ID")
	}
	m.mu.Lock()
	rec, exists := m.sandboxes[sandboxID]
	if !exists {
		residual := false
		for _, path := range []string{filepath.Dir(m.RootfsPath(sandboxID)), m.CgroupPath(sandboxID)} {
			if _, err := os.Lstat(path); err == nil {
				residual = true
			} else if !os.IsNotExist(err) {
				m.mu.Unlock()
				return err
			}
		}
		if !residual {
			m.mu.Unlock()
			return nil
		}
		// An absent RAM record is not proof that disk/cgroup resources are gone.
		rec = &SandboxRecord{ID: sandboxID, State: "cleanup_failed"}
		m.sandboxes[sandboxID] = rec
	}
	m.mu.Unlock()
	rec.opMu.Lock()
	defer rec.opMu.Unlock()
	m.mu.RLock()
	current := m.sandboxes[sandboxID] == rec
	m.mu.RUnlock()
	if !current {
		return nil
	} // another stop already completed
	return m.destroyRecord(ctx, rec)
}

// destroyRecord requires rec.opMu. It is shared by rollback, retries and all stops.
func (m *RuntimeManager) destroyRecord(ctx context.Context, rec *SandboxRecord) error {
	m.mu.Lock()
	rec.State = "stopping"
	m.revision++
	rec.Revision = m.revision
	m.mu.Unlock()
	var errs []error
	if rec.cmd != nil && rec.cmd.Process != nil {
		_ = rec.cmd.Process.Kill()
	}
	// cgroup.kill also handles processes surviving a daemon crash.
	if err := m.cgroups.Destroy(rec.ID); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if rec.waitDone != nil {
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-rec.waitDone:
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		case <-timer.C:
			errs = append(errs, errors.New("process exit timeout"))
		}
		timer.Stop()
	}
	if len(errs) == 0 {
		if err := m.unmountContainerFilesystems(ctx, m.RootfsPath(rec.ID)); err != nil {
			errs = append(errs, err)
		}
		if m.netTeardown != nil {
			if err := m.netTeardown(ctx, rec.SlotIndex, rec.ID); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			if err := m.btrfs.DestroySnapshot(ctx, rec.ID); err != nil {
				errs = append(errs, err)
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revision++
	if err := errors.Join(errs...); err != nil {
		rec.State = "cleanup_failed"
		rec.CleanupError = err.Error()
		rec.Revision = m.revision
		return fmt.Errorf("destroy sandbox %s: %w", rec.ID, err)
	}
	delete(m.sandboxes, rec.ID)
	return nil
}

func (m *RuntimeManager) unmountContainerFilesystems(ctx context.Context, rootfsPath string) error {
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
			out, err := exec.CommandContext(ctx, "umount", "-l", mp).CombinedOutput()
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

func snapshotOf(s *SandboxRecord) SandboxSnapshot {
	return SandboxSnapshot{IdleSeconds: int64(s.idleLimit / time.Second), ID: s.ID, UserID: s.UserID, SlotIndex: s.SlotIndex, IP: s.IP, PID: s.PID, State: s.State, Revision: s.Revision, StartedAt: s.StartedAt.Unix(), LastActiveAt: s.LastActiveAt.Unix(), ExpiresAt: s.ExpiresAt.Unix(), CleanupError: s.CleanupError}
}

func (m *RuntimeManager) GetSnapshot(id string) (SandboxSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sandboxes[id]
	if !ok {
		return SandboxSnapshot{}, false
	}
	return snapshotOf(s), true
}

func (m *RuntimeManager) Inventory() (uint64, []protocol.SandboxSummary) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]protocol.SandboxSummary, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		res = append(res, protocol.SandboxSummary{ID: s.ID, LastActiveAt: s.LastActiveAt.Unix(), State: s.State})
	}
	return m.revision, res
}

func (m *RuntimeManager) ListSnapshots() []SandboxSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]SandboxSnapshot, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		res = append(res, snapshotOf(s))
	}
	return res
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
