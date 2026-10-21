package runtime

import (
	"attackbox/protocol"
)

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
