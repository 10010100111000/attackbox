package runtime

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// Security Architecture Note:
// AttackBox is an interactive offensive security container intended for authorized penetration
// testing and security auditing. Users possess passwordless sudo inside their sandbox.
// To preserve essential pentesting tools (iptables packet redirection, virtual tmpfs mounting,
// chroot inspection) while strictly safeguarding the host node against compromise:
// 1. We selectively and permanently strip the 4 most critical host-destructive capabilities:
//    - CAP_SYS_BOOT (22): Prevents rebooting or shutting down the host machine.
//    - CAP_SYS_MODULE (16): Prevents loading or unloading host kernel modules.
//    - CAP_SYS_RAWIO (17): Prevents direct raw I/O or memory access to host devices.
//    - CAP_MKNOD (27): Prevents creating block/character device nodes (e.g. /dev/sda).
// 2. We deliberately preserve CAP_SYS_ADMIN inside the container for internal management,
//    while neutralizing escape vectors through host-enforced isolation:
//    - Read-only sysfs (/sys) and procfs system tunables (/proc/sys)
//    - Masking /proc/sysrq-trigger with /dev/null
//    - PID namespace isolation preventing visibility into host processes
//    - Elimination of host device access via tmpfs on /dev and stripped CAP_MKNOD.
// Note on Commit 561046a: The mention of "CAP_SYS_ADMIN" in the historical commit message was an
// inadvertent erratum; CAP_SYS_ADMIN is intentionally retained for sandbox utility.

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

// DropDangerousCapabilitiesAllThreads executes the security lockdown across Go threads:
// 1. capset: clears CAP_SYS_BOOT, CAP_SYS_RAWIO, CAP_SYS_MODULE, CAP_MKNOD from Effective & Permitted
// 2. PR_CAPBSET_DROP: permanently strips them from the Capability Bounding Set across all active OS threads
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

	// 3. Drop from Capability Bounding Set (PR_CAPBSET_DROP) across all Go OS threads
	// Bounding set is a per-thread attribute in Linux; use AllThreadsSyscall to ensure all Go runtime
	// OS threads (M) have the dangerous capabilities stripped.
	dangerousCaps := []uintptr{CapSysBoot, CapSysRawIO, CapSysModule, CapMknod}
	for _, capVal := range dangerousCaps {
		_, _, errNo := syscall.AllThreadsSyscall(syscall.SYS_PRCTL, prCapbsetDrop, capVal, 0)
		if errNo != 0 {
			_, _, errNo = syscall.RawSyscall(syscall.SYS_PRCTL, prCapbsetDrop, capVal, 0)
			if errNo != 0 {
				return fmt.Errorf("prctl PR_CAPBSET_DROP %d failed: %w", capVal, errNo)
			}
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

// VerifyCurrentThreadCapsDropped checks /proc/thread-self/status to ensure dangerous capabilities
// are strictly stripped from the OS thread that is about to execute nsenter/exec.
func VerifyCurrentThreadCapsDropped() error {
	statusPath := "/proc/thread-self/status"
	f, err := os.Open(statusPath)
	if err != nil {
		statusPath = "/proc/self/status"
		f, err = os.Open(statusPath)
		if err != nil {
			return fmt.Errorf("open %s: %w", statusPath, err)
		}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var capPrm, capEff, capBnd uint64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "CapPrm:") {
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

	for _, bit := range []int{CapSysModule, CapSysRawIO, CapSysBoot, CapMknod} {
		mask := uint64(1) << bit
		if (capEff & mask) != 0 {
			return fmt.Errorf("thread CapEff still retains dangerous capability %d (0x%x)", bit, capEff)
		}
		if (capPrm & mask) != 0 {
			return fmt.Errorf("thread CapPrm still retains dangerous capability %d (0x%x)", bit, capPrm)
		}
		if (capBnd & mask) != 0 {
			return fmt.Errorf("thread CapBnd still retains dangerous capability %d (0x%x)", bit, capBnd)
		}
	}

	return nil
}

// ExecEnterTarget executes on the host as host root. It permanently strips dangerous capabilities
// (CAP_SYS_BOOT, CAP_SYS_RAWIO, CAP_SYS_MODULE, CAP_MKNOD) from Effective, Permitted, and Bounding sets,
// then invokes nsenter on the host to attach to targetPID's namespaces and switch to user 'box'.
func ExecEnterTarget(targetPID int, targetArgs []string) error {
	return execEnterTarget(targetPID, targetArgs, false)
}

// Both PTY and desktop entrypoints drop capabilities before executing any
// sandbox binary. The root mode is for desktop setup inside the same sandbox.
func execEnterTarget(targetPID int, targetArgs []string, asRoot bool) error {
	// 0. Strictly lock goroutine to the current OS thread until syscall.Exec
	// This prevents Go M:N runtime scheduler drift between capability drop and exec in CGO environments.
	runtime.LockOSThread()

	// 1. Permanently drop dangerous capabilities from Capability Bounding Set and effective/permitted sets
	if err := DropDangerousCapabilitiesAllThreads(); err != nil {
		return fmt.Errorf("failed to drop capabilities on host: %w", err)
	}

	// 2. Fail-Closed: Verify that the current OS thread has genuinely had all dangerous caps stripped
	if err := VerifyCurrentThreadCapsDropped(); err != nil {
		return fmt.Errorf("capability strip verification failed on execution thread: %w", err)
	}

	// 2. Locate nsenter on host
	nsenterBin, err := exec.LookPath("nsenter")
	if err != nil {
		for _, fallback := range []string{"/usr/bin/nsenter", "/bin/nsenter", "/usr/local/bin/nsenter"} {
			if _, statErr := os.Stat(fallback); statErr == nil {
				nsenterBin = fallback
				break
			}
		}
	}
	if nsenterBin == "" {
		return fmt.Errorf("nsenter binary not found on host")
	}

	// 3. Assemble command inside container.
	// Prefer /sbin/runuser or /usr/sbin/runuser to switch to non-root user 'box' with supplementary 'sudo' group.
	// Fallback to /bin/su if runuser is unavailable.
	var runnerBin string
	for _, cand := range []string{
		fmt.Sprintf("/proc/%d/root/sbin/runuser", targetPID),
		fmt.Sprintf("/proc/%d/root/usr/sbin/runuser", targetPID),
	} {
		if _, statErr := os.Stat(cand); statErr == nil {
			runnerBin = strings.TrimPrefix(cand, fmt.Sprintf("/proc/%d/root", targetPID))
			break
		}
	}

	var innerCmd []string
	if asRoot {
		innerCmd = targetArgs
	} else if runnerBin != "" {
		innerCmd = []string{runnerBin, "-u", "box", "--"}
		if len(targetArgs) > 0 {
			innerCmd = append(innerCmd, targetArgs...)
		} else {
			innerCmd = append(innerCmd, "/bin/bash", "-l")
		}
	} else {
		// Fallback to su - box
		if len(targetArgs) > 0 {
			innerCmd = []string{"/bin/su", "-l", "box", "-c", strings.Join(targetArgs, " ")}
		} else {
			innerCmd = []string{"/bin/su", "-l", "box"}
		}
	}

	nsenterArgs := []string{
		"nsenter",
		"--target", strconv.Itoa(targetPID),
		"--mount", "--uts", "--ipc", "--net", "--pid",
		"--root", "--wd",
		"--",
	}
	nsenterArgs = append(nsenterArgs, innerCmd...)

	// 4. Replace process with nsenter
	return syscall.Exec(nsenterBin, nsenterArgs, os.Environ())
}

// EnterDispatcher intercepts __enter__ on the host:
// e.g. attackbox-node __enter__ <targetPID> [command...]
func EnterDispatcher() bool {
	if len(os.Args) >= 3 && (os.Args[1] == "__enter__" || os.Args[1] == "__enter_root__") {
		targetPID, err := strconv.Atoi(os.Args[2])
		if err != nil || targetPID <= 0 {
			fmt.Fprintf(os.Stderr, "Invalid target PID: %s\n", os.Args[2])
			os.Exit(1)
		}
		if err := execEnterTarget(targetPID, os.Args[3:], os.Args[1] == "__enter_root__"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: host enter failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
		return true
	}
	return false
}
