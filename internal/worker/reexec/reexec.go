package reexec

import (
	"fmt"
	"os"

	"attackbox/internal/worker/runtime"
)

// Dispatch evaluates command-line arguments to intercept hidden container entrypoints (__init__, __enter__).
// If a hidden command matches, it runs the entrypoint and terminates the process immediately.
// Returns false if normal worker service execution should proceed.
func Dispatch() bool {
	if len(os.Args) < 2 {
		return false
	}

	switch os.Args[1] {
	case "__init__":
		// Usage: /proc/self/exe __init__ <sandbox_id> <rootfs_path> <tmp_size> <run_size>
		if len(os.Args) < 6 {
			fmt.Fprintf(os.Stderr, "Error: insufficient arguments for __init__ (need id, rootfs, tmpSize, runSize)\n")
			os.Exit(1)
		}
		sandboxID := os.Args[2]
		rootfsPath := os.Args[3]
		tmpSize := os.Args[4]
		runSize := os.Args[5]

		if err := runtime.RunContainerInit(sandboxID, rootfsPath, tmpSize, runSize); err != nil {
			fmt.Fprintf(os.Stderr, "Error: container init failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
		return true

	case "__enter__":
		// Usage: /proc/self/exe __enter__ <sandbox_id> <target_pid> [cmd...]
		// Handled via nsenter or custom exec bridge
		return false

	default:
		return false
	}
}
