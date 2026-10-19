package interactive

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/creack/pty"
)

// PTYSession encapsulates an active pseudo-terminal attached to a sandbox.
type PTYSession struct {
	Master *os.File
	Cmd    *exec.Cmd
	mu     sync.Mutex
	closed bool
}

// StartPTY spawns an interactive shell inside the sandbox's namespaces via nsenter
// and binds it to a newly allocated pseudo-terminal.
func StartPTY(targetPID int, cgroupPath string, rows, cols uint16) (*PTYSession, error) {
	if rows == 0 {
		rows = 35
	}
	if cols == 0 {
		cols = 110
	}

	// Use nsenter to penetrate into all namespaces of the target sandbox PID
	cmd := exec.Command("/usr/bin/nsenter",
		"--target", strconv.Itoa(targetPID),
		"--mount", "--uts", "--ipc", "--net", "--pid",
		"--root", "--wd",
		"/bin/bash", "-l",
	)

	cmd.Env = []string{
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"USER=root",
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, fmt.Errorf("pty.StartWithSize: %w", err)
	}

	// Attach the new shell process to the sandbox's Cgroup v2 slice
	if cgroupPath != "" && cmd.Process != nil {
		cgProcs := filepath.Join(cgroupPath, "cgroup.procs")
		_ = os.WriteFile(cgProcs, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644)
	}

	return &PTYSession{
		Master: ptmx,
		Cmd:    cmd,
	}, nil
}

// Resize updates the terminal dimensions of the PTY.
func (s *PTYSession) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.Master == nil {
		return nil
	}
	return pty.Setsize(s.Master, &pty.Winsize{Rows: rows, Cols: cols})
}

// Close cleanly terminates the PTY session and reaps the child shell process.
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
		_ = s.Cmd.Wait() // Crucial: reap child to prevent defunct zombie accumulation
	}
	return nil
}
