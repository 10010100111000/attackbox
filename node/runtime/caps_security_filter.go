package runtime

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	_LINUX_CAPABILITY_VERSION_3 = 0x20080522
	prCapbsetDrop               = 24
	prSetNoNewPrivs             = 38

	CapSysModule = 16
	CapSysRawIO  = 17
	CapSysBoot   = 22
	CapMknod     = 27
)

type capUserHeader struct {
	version uint32
	pid     int32
}

type capUserData struct {
	effective   uint32
	permitted   uint32
	inheritable uint32
}

// DropDangerousCapabilitiesAllThreads executes the security lockdown on all threads of PID 1:
// 1. capset: clears CAP_SYS_BOOT, CAP_SYS_RAWIO, CAP_SYS_MODULE, CAP_MKNOD from Effective & Permitted
// 2. PR_CAPBSET_DROP: permanently strips them from the Capability Bounding Set
func DropDangerousCapabilitiesAllThreads() error {
	var hdr capUserHeader
	hdr.version = _LINUX_CAPABILITY_VERSION_3
	hdr.pid = 0 // current thread/process

	var data [2]capUserData

	// 1. Get current capability sets
	_, _, errNo := syscall.RawSyscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0)
	if errNo != 0 {
		return fmt.Errorf("capget failed: %w", errNo)
	}

	// Mask out CAP_SYS_MODULE (16), CAP_SYS_RAWIO (17), CAP_SYS_BOOT (22), CAP_MKNOD (27)
	mask0 := uint32((1 << CapSysModule) | (1 << CapSysRawIO) | (1 << CapSysBoot) | (1 << CapMknod))
	data[0].effective &^= mask0
	data[0].permitted &^= mask0
	data[0].inheritable &^= mask0

	// 2. capset across all Go OS threads
	_, _, errNo = syscall.AllThreadsSyscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0)
	if errNo != 0 {
		// Fallback to single thread RawSyscall if AllThreadsSyscall returns ENOTSUP
		_, _, errNo = syscall.RawSyscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0)
		if errNo != 0 {
			return fmt.Errorf("capset failed: %w", errNo)
		}
	}

	// 3. Drop from Capability Bounding Set (PR_CAPBSET_DROP)
	dangerousCaps := []uintptr{CapSysBoot, CapSysRawIO, CapSysModule, CapMknod}
	for _, capVal := range dangerousCaps {
		_, _, errNo := syscall.RawSyscall(syscall.SYS_PRCTL, prCapbsetDrop, capVal, 0)
		if errNo != 0 {
			return fmt.Errorf("prctl PR_CAPBSET_DROP %d failed: %w", capVal, errNo)
		}
	}

	return nil
}

// VerifyDangerousCapsDropped inspects /proc/<pid>/status to verify capabilities and NoNewPrivs.
func VerifyDangerousCapsDropped(pid int) error {
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	f, err := os.Open(statusPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", statusPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var capPrm, capEff, capBnd uint64
	var noNewPrivs int = -1

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "NoNewPrivs:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				noNewPrivs, _ = strconv.Atoi(fields[1])
			}
		} else if strings.HasPrefix(line, "CapInh:") {
			// ignore
		} else if strings.HasPrefix(line, "CapPrm:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				capPrm, _ = strconv.ParseUint(fields[1], 16, 64)
			}
		} else if strings.HasPrefix(line, "CapEff:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				capEff, _ = strconv.ParseUint(fields[1], 16, 64)
			}
		} else if strings.HasPrefix(line, "CapBnd:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				capBnd, _ = strconv.ParseUint(fields[1], 16, 64)
			}
		}
	}

	if noNewPrivs != 0 {
		return fmt.Errorf("NoNewPrivs is unexpectedly set to %d (must be 0 for sudo support)", noNewPrivs)
	}

	checkBit := func(name string, val uint64, bit int) error {
		if (val & (1 << bit)) != 0 {
			return fmt.Errorf("%s still contains dangerous capability bit %d (0x%x)", name, bit, val)
		}
		return nil
	}

	// Verify CAP_SYS_BOOT (22), CAP_SYS_RAWIO (17), CAP_SYS_MODULE (16), CAP_MKNOD (27) are strictly 0
	for _, bit := range []int{CapSysModule, CapSysRawIO, CapSysBoot, CapMknod} {
		if err := checkBit("CapEff", capEff, bit); err != nil {
			return err
		}
		if err := checkBit("CapPrm", capPrm, bit); err != nil {
			return err
		}
		if err := checkBit("CapBnd", capBnd, bit); err != nil {
			return err
		}
	}

	return nil
}
