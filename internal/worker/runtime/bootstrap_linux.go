//go:build linux

package runtime

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// RunContainerInit executes the PID 1 initialization pipeline inside the new container namespaces.
func RunContainerInit(sandboxID, rootfsPath, tmpSize, runSize string) error {
	if tmpSize == "" {
		tmpSize = "512M"
	}
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
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		return fmt.Errorf("mount /dev tmpfs: %w", err)
	}
	setupDevNodes()

	// 7. Mount tmpfs on /tmp and /run
	_ = syscall.Mount("tmpfs", "/tmp", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, fmt.Sprintf("mode=1777,size=%s", tmpSize))
	_ = syscall.Mount("tmpfs", "/run", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, fmt.Sprintf("mode=755,size=%s", runSize))

	// 8. Set container hostname
	_ = syscall.Sethostname([]byte(sandboxID))

	// 9. Drop dangerous capabilities from Bounding Set
	dropCapabilities()

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
	// Character devices: major, minor, mode
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
	}

	// Mount devpts for terminal emulation
	_ = os.MkdirAll("/dev/pts", 0755)
	_ = syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=620")
	_ = os.Symlink("/dev/pts/ptmx", "/dev/ptmx")

	// Standard POSIX file descriptor symlinks
	_ = os.Symlink("/proc/self/fd", "/dev/fd")
	_ = os.Symlink("/proc/self/fd/0", "/dev/stdin")
	_ = os.Symlink("/proc/self/fd/1", "/dev/stdout")
	_ = os.Symlink("/proc/self/fd/2", "/dev/stderr")

	// Mount /dev/shm tmpfs
	_ = os.MkdirAll("/dev/shm", 0777)
	_ = syscall.Mount("tmpfs", "/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777,size=256M")
}

func dropCapabilities() {
	// Linux PR_CAPBSET_DROP = 24
	const prCapbsetDrop = 24
	const (
		capSysModule = 16
		capSysRawIO  = 17
		capSysBoot   = 22
	)

	_, _, _ = syscall.Syscall(syscall.SYS_PRCTL, prCapbsetDrop, capSysBoot, 0)
	_, _, _ = syscall.Syscall(syscall.SYS_PRCTL, prCapbsetDrop, capSysModule, 0)
	_, _, _ = syscall.Syscall(syscall.SYS_PRCTL, prCapbsetDrop, capSysRawIO, 0)
}

func runPID1Loop() {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGCHLD)

	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				// Container terminating
				return
			case syscall.SIGCHLD:
				// Reap all available dead child processes (zombies)
				for {
					var status syscall.WaitStatus
					pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
					if err != nil || pid <= 0 {
						break
					}
				}
			}
		case <-time.After(500 * time.Millisecond):
			// Periodic safety harvest of any untrapped zombies
			for {
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
				if err != nil || pid <= 0 {
					break
				}
			}
		}
	}
}
