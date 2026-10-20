package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "m" {
		// SYS_FINIT_MODULE on Linux amd64 is 313
		_, _, err := syscall.Syscall(313, uintptr(^uint(0)), 0, 0)
		if err == syscall.EPERM {
			fmt.Println("KERNEL_MODULE_EPERM_CONFIRMED")
			os.Exit(0)
		}
		fmt.Printf("MODULE_ERR: %v\n", err)
		os.Exit(1)
	}

	err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	if err == syscall.EPERM {
		fmt.Println("KERNEL_EPERM_CONFIRMED")
		os.Exit(0)
	}
	fmt.Printf("REBOOT_ERR: %v\n", err)
	os.Exit(1)
}
