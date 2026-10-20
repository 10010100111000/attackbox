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

// EnsureParentHierarchy prepares the base cgroup and activates controller delegation once.
func (c *CgroupManager) EnsureParentHierarchy() error {
	c.parentOnce.Do(func() {
		if err := os.MkdirAll(c.CgroupRoot, 0755); err != nil {
			c.parentErr = fmt.Errorf("mkdir cgroup root %s: %w", c.CgroupRoot, err)
			return
		}

		rootSubtree := "/sys/fs/cgroup/cgroup.subtree_control"
		_ = os.WriteFile(rootSubtree, []byte("+cpu +memory +pids +io\n"), 0644)

		localSubtree := filepath.Join(c.CgroupRoot, "cgroup.subtree_control")
		_ = os.WriteFile(localSubtree, []byte("+cpu +memory +pids +io\n"), 0644)
	})

	return c.parentErr
}

// Create provisions the instance cgroup directory, writes limit parameters, and opens cgroup.procs fd.
func (c *CgroupManager) Create(sandboxID string, limits ResourceLimits) (*os.File, string, error) {
	if err := c.EnsureParentHierarchy(); err != nil {
		return nil, "", err
	}

	cgPath := filepath.Join(c.CgroupRoot, sandboxID)
	if err := os.Mkdir(cgPath, 0755); err != nil && !os.IsExist(err) {
		return nil, "", fmt.Errorf("mkdir cgroup %s: %w", cgPath, err)
	}

	// 1. Process limits (Fork bomb prevention)
	if limits.PIDsMax > 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "pids.max"), []byte(strconv.FormatInt(limits.PIDsMax, 10)+"\n"), 0644)
	}

	// 2. Memory limits (Strict Fail-Closed: Never leave unbound)
	memBytes := limits.MemoryMaxBytes
	if memBytes <= 0 {
		memBytes = int64(protocol.DefaultMemLimitMB) * 1024 * 1024
	}
	_ = os.WriteFile(filepath.Join(cgPath, "memory.max"), []byte(strconv.FormatInt(memBytes, 10)+"\n"), 0644)
	if limits.SwapMaxBytes >= 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "memory.swap.max"), []byte(strconv.FormatInt(limits.SwapMaxBytes, 10)+"\n"), 0644)
	}

	// 3. CPU Weight
	if limits.CPUWeight > 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "cpu.weight"), []byte(strconv.FormatUint(limits.CPUWeight, 10)+"\n"), 0644)
	}

	// 4. Block IO Limits
	if limits.RootDeviceNumber != "" {
		if limits.IOWriteBytesSec > 0 {
			ioMaxStr := fmt.Sprintf("%s wbps=%d\n", limits.RootDeviceNumber, limits.IOWriteBytesSec)
			_ = os.WriteFile(filepath.Join(cgPath, "io.max"), []byte(ioMaxStr), 0644)
		}
		if limits.IOWriteIOPS > 0 {
			ioMaxStr := fmt.Sprintf("%s wiops=%d\n", limits.RootDeviceNumber, limits.IOWriteIOPS)
			_ = os.WriteFile(filepath.Join(cgPath, "io.max"), []byte(ioMaxStr), 0644)
		}
	}

	// 5. Open cgroup v2 directory with O_RDONLY for CLONE_INTO_CGROUP
	cgFile, err := os.Open(cgPath)
	if err != nil {
		_ = os.Remove(cgPath)
		return nil, "", fmt.Errorf("open cgroup directory %s: %w", cgPath, err)
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
