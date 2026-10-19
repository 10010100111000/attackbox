//go:build !linux

package runtime

import (
	"os"
	"os/exec"
)

func configureSysProcAttr(cmd *exec.Cmd, cgFile *os.File) {
	// No-op for non-Linux platforms
}
