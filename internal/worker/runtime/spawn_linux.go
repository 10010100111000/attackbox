//go:build linux

package runtime

import (
	"os"
	"os/exec"
	"syscall"
)

func configureSysProcAttr(cmd *exec.Cmd, cgFile *os.File) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Five core namespaces: PID, Mount, UTS, IPC, Net
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		// Native Linux 5.7+ CLONE_INTO_CGROUP
		UseCgroupFD: true,
		CgroupFD:    int(cgFile.Fd()),
	}
}
