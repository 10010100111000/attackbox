package node

import (
	"fmt"
	"github.com/creack/pty"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

type PTYSession struct {
	Master *os.File
	Cmd    *exec.Cmd
	mu     sync.Mutex
	closed bool
}

func StartPTY(targetPID int, cgroupPath string, rows, cols uint16) (*PTYSession, error) {
	if rows == 0 {
		rows = 35
	}
	if cols == 0 {
		cols = 110
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = "/proc/self/exe"
	}

	// 1. Locate trusted host-side privilege dropping wrapper:
	// Priority 1: /usr/local/bin/attackbox-enter or PATH on host
	// Priority 2: Currently running trusted host binary via __enter__ dispatcher
	// NEVER execute any binary from container rootfs, and FAIL-CLOSED if wrapper is unavailable.
	var enterCmd string
	var cmdArgs []string

	if fallback, statErr := os.Stat("/usr/local/bin/attackbox-enter"); statErr == nil && !fallback.IsDir() {
		enterCmd = "/usr/local/bin/attackbox-enter"
		cmdArgs = []string{strconv.Itoa(targetPID)}
	} else if lookPath, lookErr := exec.LookPath("attackbox-enter"); lookErr == nil {
		enterCmd = lookPath
		cmdArgs = []string{strconv.Itoa(targetPID)}
	} else if exePath != "" {
		enterCmd = exePath
		cmdArgs = []string{"__enter__", strconv.Itoa(targetPID)}
	} else {
		return nil, fmt.Errorf("fail-closed: no trusted host attackbox-enter wrapper available")
	}

	cmd := exec.Command(enterCmd, cmdArgs...)
	cmd.Env = []string{
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/home/box",
		"USER=box",
		"LOGNAME=box",
		"SHELL=/bin/bash",
	}

	// 2. Open cgroup file descriptor for native Linux 5.7+ CLONE_INTO_CGROUP
	// Ensures child process is born directly inside the sandbox cgroup, eliminating race window.
	var cgFile *os.File
	if cgroupPath != "" {
		var openErr error
		cgFile, openErr = os.Open(cgroupPath)
		if openErr != nil {
			return nil, fmt.Errorf("open sandbox cgroup %s failed: %w", cgroupPath, openErr)
		}
		defer cgFile.Close()

		cmd.SysProcAttr = &syscall.SysProcAttr{
			UseCgroupFD: true,
			CgroupFD:    int(cgFile.Fd()),
		}
	}

	// 3. Start PTY session with atomic cgroup assignment
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, fmt.Errorf("pty.StartWithSize: %w", err)
	}

	// 4. Secondary cgroup.procs sync with strict error checking (Fail-Closed)
	if cgroupPath != "" && cmd.Process != nil {
		cgProcs := filepath.Join(cgroupPath, "cgroup.procs")
		if writeErr := os.WriteFile(cgProcs, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); writeErr != nil {
			// Fail-closed: kill process immediately, close ptmx, and return error
			_ = cmd.Process.Kill()
			_ = ptmx.Close()
			_ = cmd.Wait()
			return nil, fmt.Errorf("failed to bind PTY process %d to cgroup %s: %w", cmd.Process.Pid, cgroupPath, writeErr)
		}
	}

	return &PTYSession{
		Master: ptmx,
		Cmd:    cmd,
	}, nil
}

func (s *PTYSession) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.Master == nil {
		return nil
	}
	return pty.Setsize(s.Master, &pty.Winsize{Rows: rows, Cols: cols})
}

func (s *PTYSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	if s.Cmd != nil && s.Cmd.Process != nil {
		_ = s.Cmd.Process.Kill()
	}
	if s.Master != nil {
		_ = s.Master.Close()
	}
	if s.Cmd != nil {
		_ = s.Cmd.Wait()
	}
	return nil
}
