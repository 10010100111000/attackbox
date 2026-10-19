package main

import (
	"fmt"
	"os"
	"syscall"

	"attackbox/node/runtime"
)

func main() {
	// 1. Drop dangerous capabilities (CAP_SYS_BOOT, CAP_SYS_RAWIO, CAP_SYS_MODULE) from Effective, Permitted, Bounding
	// and lock down privileges with PR_SET_NO_NEW_PRIVS
	if err := runtime.DropDangerousCapabilitiesAllThreads(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: attackbox-enter failed to drop capabilities: %v\n", err)
		os.Exit(1)
	}

	// 2. Resolve target command to execute (default: /bin/bash -l)
	targetCmd := "/bin/bash"
	targetArgs := []string{"/bin/bash", "-l"}

	if len(os.Args) > 1 {
		targetCmd = os.Args[1]
		targetArgs = os.Args[1:]
	}

	env := os.Environ()

	// 3. Execvp into target shell / command, replacing current process
	if err := syscall.Exec(targetCmd, targetArgs, env); err != nil {
		fmt.Fprintf(os.Stderr, "Error: attackbox-enter exec %s failed: %v\n", targetCmd, err)
		os.Exit(1)
	}
}
