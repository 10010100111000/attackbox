package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"attackbox/internal/worker/config"
	"attackbox/internal/worker/reexec"
	"attackbox/internal/worker/runtime"
)

func main() {
	// Step 1: Intercept hidden container re-exec entrypoints (__init__, __enter__)
	// This must execute before any Go initialization, networking, or config parsing.
	if reexec.Dispatch() {
		return
	}

	configPath := flag.String("config", "configs/worker.yaml", "Path to worker configuration file")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("    AttackBox Sacrificial Worker Node (v2.0)      ")
	fmt.Println("==================================================")

	// Load configuration (all limits, paths, and thresholds are fully adjustable)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("[INFO] %v\n", err)
	}

	fmt.Printf("[CONFIG] Node ID: %s\n", cfg.Node.NodeID)
	fmt.Printf("[CONFIG] Data Server: %s (TLS: %v)\n", cfg.DataServer.ListenAddr, cfg.DataServer.TLSEnabled)
	fmt.Printf("[CONFIG] Btrfs Base Rootfs: %s\n", cfg.Runtime.BaseRootfs)
	fmt.Printf("[CONFIG] Cgroups v2 Root: %s\n", cfg.Runtime.CgroupRoot)
	fmt.Printf("[CONFIG] Limits: PIDsMax=%d, MemMax=%dMB, Swap=%dMB, CPUWeight=%d, IOWriteMax=%d B/s\n",
		cfg.Runtime.Limits.PIDsMax,
		cfg.Runtime.Limits.MemoryMaxMB,
		cfg.Runtime.Limits.MemorySwapMaxMB,
		cfg.Runtime.Limits.CPUWeight,
		cfg.Runtime.Limits.IOWriteBytesSec,
	)
	fmt.Printf("[CONFIG] Disk Guard: Enabled=%v, MinFree=%dMB, WarnFree=%dMB\n",
		cfg.Runtime.DiskGuard.Enabled,
		cfg.Runtime.DiskGuard.MinFreeSpaceMB,
		cfg.Runtime.DiskGuard.WarnFreeSpaceMB,
	)

	// Initialize runtime manager
	rtMgr := runtime.NewRuntimeManager(cfg.Runtime)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rtMgr.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Failed to start runtime manager: %v\n", err)
		os.Exit(1)
	}
	defer rtMgr.Stop()

	fmt.Printf("[INFO] Worker runtime initialized successfully. Host free disk: %d MB\n",
		rtMgr.DiskGuard().GetCurrentFreeMB())

	// Wait for OS termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[INFO] Shutting down worker gracefully...")
}
