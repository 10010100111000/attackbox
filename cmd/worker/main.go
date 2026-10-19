package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"attackbox/internal/worker/agent"
	"attackbox/internal/worker/config"
	"attackbox/internal/worker/dataserver"
	"attackbox/internal/worker/network"
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
	fmt.Printf("[CONFIG] Control WSS: %s\n", cfg.Node.ControlURL)
	fmt.Printf("[CONFIG] Data Server: %s (TLS: %v)\n", cfg.DataServer.ListenAddr, cfg.DataServer.TLSEnabled)
	fmt.Printf("[CONFIG] Btrfs Base Rootfs: %s\n", cfg.Runtime.BaseRootfs)
	fmt.Printf("[CONFIG] Cgroups v2 Root: %s\n", cfg.Runtime.CgroupRoot)
	fmt.Printf("[CONFIG] Baseline Limits: PIDsMax=%d, MemMax=%dMB, Swap=%dMB, CPUWeight=%d, IOWriteMax=%d B/s\n",
		cfg.Runtime.Limits.PIDsMax,
		cfg.Runtime.Limits.MemoryMaxMB,
		cfg.Runtime.Limits.MemorySwapMaxMB,
		cfg.Runtime.Limits.CPUWeight,
		cfg.Runtime.Limits.IOWriteBytesSec,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Initialize Host Network & Firewall Baseline
	if err := network.EnsureHostNetwork(cfg.Network); err != nil {
		log.Printf("[WARN] Host network init warning: %v (running non-root or lab test?)", err)
	} else {
		log.Printf("[INFO] Host bridge %s and nftables containment baseline initialized", cfg.Network.BridgeName)
	}

	// 2. Initialize Container Runtime Manager
	rtMgr := runtime.NewRuntimeManager(cfg.Runtime)
	if err := rtMgr.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Failed to start runtime manager: %v\n", err)
		os.Exit(1)
	}
	defer rtMgr.Stop()

	fmt.Printf("[INFO] Worker runtime initialized successfully. Host free disk: %d MB\n",
		rtMgr.DiskGuard().GetCurrentFreeMB())

	// 3. Initialize & Start Direct Data Server (:8443)
	dataSrv, err := dataserver.NewServer(cfg.DataServer, rtMgr)
	if err != nil {
		log.Printf("[WARN] Failed to configure Data Server: %v", err)
	} else {
		go func() {
			log.Printf("[INFO] Direct Data Server listening on %s", cfg.DataServer.ListenAddr)
			if err := dataSrv.Start(); err != nil {
				log.Printf("[DATA] Data server stopped: %v", err)
			}
		}()
		defer func() {
			_ = dataSrv.Stop(context.Background())
		}()
	}

	// 4. Initialize & Start Reverse Agent to Control Hub
	workerAgent := agent.NewAgent(cfg, rtMgr, func(pubKey []byte) {
		if dataSrv != nil {
			dataSrv.SetPublicKey(pubKey)
		}
	})
	workerAgent.Start(ctx)
	defer workerAgent.Stop()

	// Wait for OS termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[INFO] Shutting down worker gracefully...")
}
