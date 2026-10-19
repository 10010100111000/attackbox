package runtime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CgroupManager manages Cgroups v2 directories, limits, and nuclear teardown.
type CgroupManager struct {
	CgroupRoot string
}

// NewCgroupManager creates a Cgroups v2 controller.
func NewCgroupManager(root string) *CgroupManager {
	if root == "" {
		root = "/sys/fs/cgroup/attackbox"
	}
	return &CgroupManager{CgroupRoot: root}
}

// EnsureParentHierarchy prepares the base cgroup and activates controller delegation.
func (c *CgroupManager) EnsureParentHierarchy() error {
	if err := os.MkdirAll(c.CgroupRoot, 0755); err != nil {
		return fmt.Errorf("mkdir cgroup root %s: %w", c.CgroupRoot, err)
	}

	// Enable controllers in root subtree_control
	_ = os.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte("+cpu +memory +pids +io"), 0644)

	// Enable controllers in attackbox subtree_control for sandbox delegation
	subtreePath := filepath.Join(c.CgroupRoot, "cgroup.subtree_control")
	_ = os.WriteFile(subtreePath, []byte("+cpu +memory +pids +io"), 0644)

	return nil
}

// Create creates a dedicated cgroup directory for a sandbox, writes resource limits,
// and returns an open file descriptor for CLONE_INTO_CGROUP.
func (c *CgroupManager) Create(sandboxID string, limits ResourceLimits) (*os.File, string, error) {
	if err := c.EnsureParentHierarchy(); err != nil {
		return nil, "", err
	}

	cgPath := filepath.Join(c.CgroupRoot, sandboxID)
	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return nil, "", fmt.Errorf("mkdir sandbox cgroup %s: %w", cgPath, err)
	}

	// 1. Process limits (pids.max)
	if limits.PIDsMax > 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "pids.max"), []byte(strconv.FormatInt(limits.PIDsMax, 10)), 0644)
	}

	// 2. Memory limits (memory.max & memory.swap.max)
	if limits.MemoryMaxBytes > 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "memory.max"), []byte(strconv.FormatInt(limits.MemoryMaxBytes, 10)), 0644)
	}
	if limits.SwapMaxBytes >= 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "memory.swap.max"), []byte(strconv.FormatInt(limits.SwapMaxBytes, 10)), 0644)
	}

	// 3. CPU scheduling weight (cpu.weight)
	weight := limits.CPUWeight
	if weight == 0 {
		weight = 100
	}
	_ = os.WriteFile(filepath.Join(cgPath, "cpu.weight"), []byte(strconv.FormatUint(weight, 10)), 0644)

	// 4. Disk write rate limits (io.max)
	if limits.IOWriteBytesSec > 0 || limits.IOWriteIOPS > 0 {
		devNum := limits.RootDeviceNumber
		if devNum == "" {
			devNum = detectRootDeviceMajorMinor()
		}
		if devNum != "" {
			var ioParts []string
			if limits.IOWriteBytesSec > 0 {
				ioParts = append(ioParts, fmt.Sprintf("wbps=%d", limits.IOWriteBytesSec))
			}
			if limits.IOWriteIOPS > 0 {
				ioParts = append(ioParts, fmt.Sprintf("wiops=%d", limits.IOWriteIOPS))
			}
			if len(ioParts) > 0 {
				rule := fmt.Sprintf("%s %s\n", devNum, strings.Join(ioParts, " "))
				_ = os.WriteFile(filepath.Join(cgPath, "io.max"), []byte(rule), 0644)
			}
		}
	}

	// 5. Open directory handle for CLONE_INTO_CGROUP
	cgFile, err := os.Open(cgPath)
	if err != nil {
		return nil, "", fmt.Errorf("open cgroup directory: %w", err)
	}

	return cgFile, cgPath, nil
}

// Kill triggers cgroup.kill to atomically terminate all processes inside the sandbox.
// It waits up to 2 seconds for cgroup.procs to drain completely.
func (c *CgroupManager) Kill(sandboxID string) error {
	cgPath := filepath.Join(c.CgroupRoot, sandboxID)
	killFile := filepath.Join(cgPath, "cgroup.kill")
	if _, err := os.Stat(killFile); err != nil {
		// Cgroup doesn't exist or already removed
		return nil
	}

	// Write 1 to cgroup.kill
	if err := os.WriteFile(killFile, []byte("1"), 0644); err != nil {
		return fmt.Errorf("write cgroup.kill: %w", err)
	}

	// Wait for all processes to terminate (cgroup.procs empty)
	procsFile := filepath.Join(cgPath, "cgroup.procs")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(procsFile)
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			// All processes cleanly exited
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

// Destroy cleans up the cgroup directory after processes have exited.
func (c *CgroupManager) Destroy(sandboxID string) error {
	cgPath := filepath.Join(c.CgroupRoot, sandboxID)
	return os.Remove(cgPath)
}

// detectRootDeviceMajorMinor parses /proc/self/mountinfo to discover the root '/' major:minor.
func detectRootDeviceMajorMinor() string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "254:1" // Fallback default
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// mountinfo format: id parent_id major:minor root mount_point ...
		if len(fields) >= 5 && fields[4] == "/" {
			return fields[2]
		}
	}
	return "254:1"
}
