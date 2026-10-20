package main

import (
	"fmt"
	"os"
	"strconv"

	"attackbox/node/runtime"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <targetPID> [command...]\n", os.Args[0])
		os.Exit(1)
	}

	targetPID, err := strconv.Atoi(os.Args[1])
	if err != nil || targetPID <= 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid target PID '%s'\n", os.Args[1])
		os.Exit(1)
	}

	if err := runtime.ExecEnterTarget(targetPID, os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: attackbox-enter failed: %v\n", err)
		os.Exit(1)
	}
}
