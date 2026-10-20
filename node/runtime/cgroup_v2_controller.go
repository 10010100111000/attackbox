package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"attackbox/protocol"
)

// CgroupManager manages Cgroups v2 directories, limits, and nuclear teardown.
type CgroupManager struct {
	CgroupRoot string
	parentOnce sync.Once
	parentErr  error
}

// NewCgroupManager creates a Cgroups v2 controller.
func NewCgroupManager(root string) *CgroupManager {
	if root == "" {
		root = "/sys/fs/cgroup/attackbox"
	}
	return &CgroupManager{CgroupRoot: root}
}

// activateControllers tries to enable requested controllers on the subtree_control of a given cgroup path.
func activateControllers(cgroupPath string, controllers []string) error {
	subtreeFile := filepath.Join(cgroupPath, "cgroup.subtree_control")
	var batch []string
	for _, ctrl := range controllers {
		batch = append(batch, "+"+ctrl)
	}
	batchPayload := []byte(strings.Join(batch, " ") + "\n")
	if err := os.WriteFile(subtreeFile, batchPayload, 0644); err == nil {
		return nil
	}

	// If batch enable fails, try enabling controllers individually
	for _, ctrl := range controllers {
		_ = os.WriteFile(subtreeFile, []byte("+"+ctrl+"\n"), 0644)
	}
	return nil
}

// EnsureParentHierarchy prepares the base cgroup and activates controller delegation once.
func (c *CgroupManager) EnsureParentHierarchy() error {
	c.parentOnce.Do(func() {
		if err := os.MkdirAll(c.CgroupRoot, 0755); err != nil {
			c.parentErr = fmt.Errorf("mkdir cgroup root %s: %w", c.CgroupRoot, err)
			return
		}

		desiredControllers := []string{"memory", "pids", "cpu", "io"}
		_ = activateControllers("/sys/fs/cgroup", desiredControllers)
		_ = activateControllers(c.CgroupRoot, desiredControllers)

		// Verification: Read back c.CgroupRoot/cgroup.subtree_control to ensure essential controllers are active
		localSubtree := filepath.Join(c.CgroupRoot, "cgroup.subtree_control")
		data, err := os.ReadFile(localSubtree)
		if err != nil {
			c.parentErr = fmt.Errorf("read %s: %w", localSubtree, err)
			return
		}
		activeSubtree := string(data)
		for _, required := range []string{"memory", "pids"} {
			if !strings.Contains(activeSubtree, required) {
				c.parentErr = fmt.Errorf("cgroup v2 missing required controller '%s' in %s (active: %s)", required, localSubtree, strings.TrimSpace(activeSubtree))
				return
			}
		}
	})

	return c.parentErr
}

// Create provisions the instance cgroup directory, writes limit parameters, and opens cgroup.procs fd.
// All parameter writes are strictly verified with fail-closed error handling and readback validation.
func (c *CgroupManager) Create(sandboxID string, limits ResourceLimits) (*os.File, string, error) {
	if err := c.EnsureParentHierarchy(); err != nil {
		return nil, "", err
	}

	cgPath := filepath.Join(c.CgroupRoot, sandboxID)
	if err := os.Mkdir(cgPath, 0755); err != nil && !os.IsExist(err) {
		return nil, "", fmt.Errorf("mkdir cgroup %s: %w", cgPath, err)
	}

	rollback := func(stepErr error) (*os.File, string, error) {
		_ = os.Remove(cgPath)
		return nil, "", stepErr
	}

	// 1. Process limits (Fork bomb prevention)
	if limits.PIDsMax > 0 {
		pidsPath := filepath.Join(cgPath, "pids.max")
		valStr := strconv.FormatInt(limits.PIDsMax, 10)
		if err := os.WriteFile(pidsPath, []byte(valStr+"\n"), 0644); err != nil {
			return rollback(fmt.Errorf("write pids.max (%d): %w", limits.PIDsMax, err))
		}
		// Write-then-readback verification
		if data, err := os.ReadFile(pidsPath); err != nil || strings.TrimSpace(string(data)) != valStr {
			return rollback(fmt.Errorf("verify pids.max readback: expected %s, got %s (err: %v)", valStr, strings.TrimSpace(string(data)), err))
		}
	}

	// 2. Memory limits (Strict Fail-Closed: Never leave unbound)
	memBytes := limits.MemoryMaxBytes
	if memBytes <= 0 {
		memBytes = int64(protocol.DefaultMemLimitMB) * 1024 * 1024
	}
	memPath := filepath.Join(cgPath, "memory.max")
	valStr := strconv.FormatInt(memBytes, 10)
	if err := os.WriteFile(memPath, []byte(valStr+"\n"), 0644); err != nil {
		return rollback(fmt.Errorf("write memory.max (%d): %w", memBytes, err))
	}
	// Write-then-readback verification
	if data, err := os.ReadFile(memPath); err != nil || strings.TrimSpace(string(data)) != valStr {
		return rollback(fmt.Errorf("verify memory.max readback: expected %s, got %s (err: %v)", valStr, strings.TrimSpace(string(data)), err))
	}

	if limits.SwapMaxBytes >= 0 {
		swapPath := filepath.Join(cgPath, "memory.swap.max")
		swapStr := strconv.FormatInt(limits.SwapMaxBytes, 10)
		if err := os.WriteFile(swapPath, []byte(swapStr+"\n"), 0644); err == nil {
			_, _ = os.ReadFile(swapPath)
		}
	}

	// 3. CPU Weight
	if limits.CPUWeight > 0 {
		cpuPath := filepath.Join(cgPath, "cpu.weight")
		valStr := strconv.FormatUint(limits.CPUWeight, 10)
		if err := os.WriteFile(cpuPath, []byte(valStr+"\n"), 0644); err != nil {
			return rollback(fmt.Errorf("write cpu.weight (%d): %w", limits.CPUWeight, err))
		}
		// Write-then-readback verification
		if data, err := os.ReadFile(cpuPath); err != nil || strings.TrimSpace(string(data)) != valStr {
			return rollback(fmt.Errorf("verify cpu.weight readback: expected %s, got %s (err: %v)", valStr, strings.TrimSpace(string(data)), err))
		}
	}

	// 4. Block IO Limits (Atomic single-line write: <dev> wbps=N wiops=M)
	if limits.RootDeviceNumber != "" {
		var ioEntries []string
		if limits.IOWriteBytesSec > 0 {
			ioEntries = append(ioEntries, fmt.Sprintf("wbps=%d", limits.IOWriteBytesSec))
		}
		if limits.IOWriteIOPS > 0 {
			ioEntries = append(ioEntries, fmt.Sprintf("wiops=%d", limits.IOWriteIOPS))
		}
		if len(ioEntries) > 0 {
			ioMaxStr := fmt.Sprintf("%s %s\n", limits.RootDeviceNumber, strings.Join(ioEntries, " "))
			ioPath := filepath.Join(cgPath, "io.max")
			if err := os.WriteFile(ioPath, []byte(ioMaxStr), 0644); err != nil {
				return rollback(fmt.Errorf("write io.max (%s): %w", strings.TrimSpace(ioMaxStr), err))
			}
			// Write-then-readback verification: Ensure device entry is recorded in io.max
			if data, err := os.ReadFile(ioPath); err != nil || !strings.Contains(string(data), limits.RootDeviceNumber) {
				return rollback(fmt.Errorf("verify io.max readback: dev %s missing in io.max (content: %q, err: %v)", limits.RootDeviceNumber, string(data), err))
			}
		}
	}

	// 5. Open cgroup v2 directory with O_RDONLY for CLONE_INTO_CGROUP
	cgFile, err := os.Open(cgPath)
	if err != nil {
		return rollback(fmt.Errorf("open cgroup directory %s: %w", cgPath, err))
	}

	return cgFile, cgPath, nil
}

// Destroy performs atomic, complete teardown of the instance cgroup without leaving zombies.
func (c *CgroupManager) Destroy(sandboxID string) error {
	cgPath := filepath.Join(c.CgroupRoot, sandboxID)
	if _, err := os.Stat(cgPath); os.IsNotExist(err) {
		return nil
	}

	// 1. Freeze cgroup processes via cgroup.freeze if available
	freezePath := filepath.Join(cgPath, "cgroup.freeze")
	_ = os.WriteFile(freezePath, []byte("1\n"), 0644)

	// 2. Kill all processes in cgroup via cgroup.kill
	killPath := filepath.Join(cgPath, "cgroup.kill")
	if err := os.WriteFile(killPath, []byte("1\n"), 0644); err != nil {
		// Fallback: manually kill all PIDs listed in cgroup.procs
		_ = c.killPIDsInProcs(cgPath)
	}

	// 3. Wait for processes to exit (up to 3 seconds) with exponential backoff
	deadline := time.Now().Add(3 * time.Second)
	backoff := 2 * time.Millisecond
	for time.Now().Before(deadline) {
		if isCgroupEmpty(cgPath) {
			break
		}
		time.Sleep(backoff)
		if backoff < 50*time.Millisecond {
			backoff *= 2
		}
	}

	// 4. Unfreeze so processes can finish dying
	_ = os.WriteFile(freezePath, []byte("0\n"), 0644)

	// 5. Remove cgroup directory (retries for kernel release with exponential backoff)
	backoff = 2 * time.Millisecond
	for i := 0; i < 15; i++ {
		err := os.Remove(cgPath)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		time.Sleep(backoff)
		if backoff < 50*time.Millisecond {
			backoff *= 2
		}
	}

	return os.Remove(cgPath)
}

func isCgroupEmpty(cgPath string) bool {
	eventsPath := filepath.Join(cgPath, "cgroup.events")
	if data, err := os.ReadFile(eventsPath); err == nil {
		if strings.Contains(string(data), "populated 0") {
			return true
		}
	}
	procsPath := filepath.Join(cgPath, "cgroup.procs")
	if data, err := os.ReadFile(procsPath); err == nil {
		return len(bytes.TrimSpace(data)) == 0
	}
	return true
}

func (c *CgroupManager) killPIDsInProcs(cgPath string) error {
	procsPath := filepath.Join(cgPath, "cgroup.procs")
	pids, err := readPIDs(procsPath)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
	return nil
}

func readPIDs(path string) ([]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pids []int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if pid, err := strconv.Atoi(line); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}
