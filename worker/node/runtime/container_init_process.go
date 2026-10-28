package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"attackbox/protocol"
)

// InitDispatcher intercepts the __init__ re-execution entrypoint when the binary is launched inside new namespaces.
// It parses the policy JSON from ATTACKBOX_POLICY and runs the PID 1 initialization pipeline.
func InitDispatcher() bool {
	if len(os.Args) >= 4 && os.Args[1] == "__init__" {
		sandboxID := os.Args[2]
		rootfsPath := os.Args[3]

		var policy protocol.SandboxPolicy
		if raw := os.Getenv("ATTACKBOX_POLICY"); raw != "" {
			_ = json.Unmarshal([]byte(raw), &policy)
		}

		if err := RunContainerInit(sandboxID, rootfsPath, policy); err != nil {
			fmt.Fprintf(os.Stderr, "Error: container init failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
		return true
	}
	return false
}

// configureSysProcAttr binds the container process to 5 core Linux namespaces and Cgroup v2.
func configureSysProcAttr(cmd *exec.Cmd, cgFile *os.File) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Five core namespaces: PID, Mount, UTS, IPC, Net
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNET,
		// Native Linux 5.7+ CLONE_INTO_CGROUP
		UseCgroupFD: true,
		CgroupFD:    int(cgFile.Fd()),
	}
}

// RunContainerInit executes the PID 1 initialization pipeline inside the new container namespaces.
func RunContainerInit(sandboxID, rootfsPath string, policy protocol.SandboxPolicy) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tmpSize := policy.Storage.TmpfsTmpSize
	if tmpSize == "" {
		tmpSize = "512M"
	}
	runSize := policy.Storage.TmpfsRunSize
	if runSize == "" {
		runSize = "256M"
	}

	// 1. Ensure private mount namespace
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_SLAVE, ""); err != nil {
		return fmt.Errorf("make mount slave: %w", err)
	}

	// 2. Bind mount rootfs to itself so it is recognized as a mountpoint
	if err := syscall.Mount(rootfsPath, rootfsPath, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs: %w", err)
	}

	// 3. PivotRoot
	oldRoot := filepath.Join(rootfsPath, ".oldroot")
	if err := os.MkdirAll(oldRoot, 0700); err != nil {
		return fmt.Errorf("mkdir .oldroot: %w", err)
	}
	if err := syscall.PivotRoot(rootfsPath, oldRoot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}

	// Unmount and detach old host root
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount .oldroot: %w", err)
	}
	_ = os.Remove("/.oldroot")

	// 4. Mount private /proc
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}

	// 5. Read-only mount /sys
	if err := syscall.Mount("sysfs", "/sys", "sysfs", syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount /sys: %w", err)
	}

	// 6. Mount tmpfs on /dev and configure standard character devices
	// CRITICAL SECURITY: Do NOT use MS_NODEV on /dev (required for /dev/null, /dev/urandom),
	// but strip CAP_MKNOD in Step 9 to permanently prevent creating any host device nodes (/dev/sda).
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		return fmt.Errorf("mount /dev tmpfs: %w", err)
	}
	setupDevNodes()

	// 7. Mount tmpfs on /tmp and /run
	_ = syscall.Mount("tmpfs", "/tmp", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, fmt.Sprintf("mode=1777,size=%s", tmpSize))
	_ = syscall.Mount("tmpfs", "/run", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, fmt.Sprintf("mode=755,size=%s", runSize))

	// 8. Apply dynamic filesystem security policies
	if policy.Security.ReadOnlyProcSys {
		if err := syscall.Mount("/proc/sys", "/proc/sys", "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("bind mount /proc/sys: %w", err)
		}
		if err := syscall.Mount("", "/proc/sys", "", syscall.MS_REMOUNT|syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("remount /proc/sys read-only: %w", err)
		}
	}

	if policy.Security.MaskSysrq {
		if _, err := os.Stat("/proc/sysrq-trigger"); err == nil {
			if err := syscall.Mount("/dev/null", "/proc/sysrq-trigger", "bind", syscall.MS_BIND, ""); err != nil {
				return fmt.Errorf("mask /proc/sysrq-trigger: %w", err)
			}
		}
	}

	// 9. Drop dangerous capabilities (CAP_SYS_BOOT, CAP_SYS_RAWIO, CAP_SYS_MODULE, CAP_MKNOD)
	if err := DropDangerousCapabilitiesAllThreads(); err != nil {
		return fmt.Errorf("drop capabilities failed: %w", err)
	}

	// 10. Signal ready to parent via extra pipe fd (Fd 3)
	pipeFile := os.NewFile(3, "init_pipe")
	if pipeFile != nil {
		_, _ = pipeFile.WriteString("READY\n")
		_ = pipeFile.Close()
	}

	// 11. Run PID 1 loop: reap zombies and wait for termination signal
	runPID1Loop()
	return nil
}

func setupDevNodes() {
	devices := []struct {
		path  string
		major uint32
		minor uint32
		mode  uint32
	}{
		{"/dev/null", 1, 3, 0666},
		{"/dev/zero", 1, 5, 0666},
		{"/dev/full", 1, 7, 0666},
		{"/dev/random", 1, 8, 0666},
		{"/dev/urandom", 1, 9, 0666},
		{"/dev/tty", 5, 0, 0666},
	}

	for _, dev := range devices {
		devNum := int((dev.major << 8) | dev.minor)
		_ = syscall.Mknod(dev.path, dev.mode|syscall.S_IFCHR, devNum)
		_ = syscall.Chmod(dev.path, dev.mode)
	}

	_ = os.MkdirAll("/dev/pts", 0755)
	_ = syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=620")
	_ = os.Symlink("/dev/pts/ptmx", "/dev/ptmx")

	_ = os.Symlink("/proc/self/fd", "/dev/fd")
	_ = os.Symlink("/proc/self/fd/0", "/dev/stdin")
	_ = os.Symlink("/proc/self/fd/1", "/dev/stdout")
	_ = os.Symlink("/proc/self/fd/2", "/dev/stderr")

	_ = os.MkdirAll("/dev/shm", 0777)
	_ = syscall.Mount("tmpfs", "/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777,size=256M")
}

func reapChildren() {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil || pid <= 0 {
			break
		}
	}
}

func runPID1Loop() {
	sigChan := make(chan os.Signal, 64)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGCHLD)
	defer signal.Stop(sigChan)

	reapChildren()

	for sig := range sigChan {
		switch sig {
		case syscall.SIGTERM, syscall.SIGINT:
			return
		case syscall.SIGCHLD:
			reapChildren()
		}
	}
}
