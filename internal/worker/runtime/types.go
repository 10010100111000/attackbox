package runtime

import (
	"time"

	"attackbox/internal/worker/config"
)

// ResourceLimits holds resource constraints applied to a single sandbox cgroup.
type ResourceLimits struct {
	PIDsMax          int64
	MemoryMaxBytes   int64
	SwapMaxBytes     int64
	CPUWeight        uint64
	IOWriteBytesSec  int64
	IOWriteIOPS      int64
	RootDeviceNumber string
}

// LimitsFromConfig converts configuration limits to runtime limits with byte conversions.
func LimitsFromConfig(cfg config.ResourceLimitsConfig) ResourceLimits {
	var memBytes int64
	if cfg.MemoryMaxMB > 0 {
		memBytes = cfg.MemoryMaxMB * 1024 * 1024
	}
	var swapBytes int64
	if cfg.MemorySwapMaxMB > 0 {
		swapBytes = cfg.MemorySwapMaxMB * 1024 * 1024
	}

	return ResourceLimits{
		PIDsMax:          cfg.PIDsMax,
		MemoryMaxBytes:   memBytes,
		SwapMaxBytes:     swapBytes,
		CPUWeight:        cfg.CPUWeight,
		IOWriteBytesSec:  cfg.IOWriteBytesSec,
		IOWriteIOPS:      cfg.IOWriteIOPS,
		RootDeviceNumber: cfg.RootDeviceNumber,
	}
}

// SandboxConfig holds complete parameters to instantiate a sandbox.
type SandboxConfig struct {
	ID           string
	Limits       ResourceLimits
	TmpfsTmpSize string
	TmpfsRunSize string
	Command      []string
	Env          []string
}

// SandboxState represents the active runtime state of a sandbox.
type SandboxState struct {
	ID          string        `json:"id"`
	PID         int           `json:"pid"`
	RootfsPath  string        `json:"rootfs_path"`
	CgroupPath  string        `json:"cgroup_path"`
	Limits      ResourceLimits `json:"limits"`
	StartedAt   time.Time     `json:"started_at"`
	IsExited    bool          `json:"is_exited"`
	ExitCode    int           `json:"exit_code"`
}
