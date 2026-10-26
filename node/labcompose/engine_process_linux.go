//go:build linux

package labcompose

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Compose may invoke a CLI plugin. Terminate this command's process group on
// cancellation, then bound pipe draining so inherited descriptors cannot keep a
// cancelled preparation occupying the node slot indefinitely.
func configureDockerCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 3 * time.Second
}
