package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"attackbox/node"
	"attackbox/node/runtime"
)

func main() {
	// Step 1: Intercept hidden container re-exec entrypoint (__init__)
	// This must execute before any Go initialization, networking, or config parsing.
	if runtime.InitDispatcher() {
		return
	}

	// Step 1.5: Intercept host container enter entrypoint (__enter__)
	if runtime.EnterDispatcher() {
		return
	}

	configPath := flag.String("config", "configs/node.yaml", "Path to node configuration file")
	buildBaseOnly := flag.Bool("build-base", false, "Force derive and sanitize golden base image, then exit")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("    AttackBox Controlled Node Daemon (v2.0)       ")
	fmt.Println("==================================================")

	// Load configuration from YAML
	cfg, err := node.LoadNodeConfig(*configPath)
	if err != nil {
		fmt.Printf("[INFO] %v\n", err)
	}

	if *buildBaseOnly {
		fmt.Printf("[INFO] Building and sanitizing golden base rootfs at %s...\n", cfg.Runtime.BaseRootfs)
		builder := runtime.NewGoldenImageBuilder(cfg.Runtime.BaseRootfs)
		if err := builder.EnsureGoldenImage(); err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] Build golden image failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[SUCCESS] Golden base rootfs successfully built, sanitized, and locked read-only.")
		return
	}

	log.Printf("[CONFIG] Node ID: %s", cfg.Node.NodeID)
	log.Printf("[CONFIG] Control WSS: %s", cfg.Node.ControlURL)
	log.Printf("[CONFIG] Gateway Listener: %s (TLS: %v)", cfg.Gateway.ListenAddr, cfg.Gateway.TLSEnabled)
	log.Printf("[CONFIG] Btrfs Base Rootfs: %s", cfg.Runtime.BaseRootfs)
	log.Printf("[CONFIG] Cgroups v2 Root: %s", cfg.Runtime.CgroupRoot)
	log.Printf("[CONFIG] Disk Guard: MinFreeSpace=%dMB, WarnFreeSpace=%dMB",
		cfg.Runtime.DiskGuard.MinFreeSpaceMB,
		cfg.Runtime.DiskGuard.WarnFreeSpaceMB,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Initialize Host Network & Firewall Baseline
	if err := node.EnsureHostNetwork(cfg.Network); err != nil {
		log.Printf("[WARN] Host network init warning: %v (running non-root or lab test?)", err)
	} else {
		log.Printf("[INFO] Host bridge %s and nftables containment baseline initialized", cfg.Network.BridgeName)
	}

	// 2. Initialize Container Runtime Manager
	rtMgr := runtime.NewRuntimeManager(cfg.Runtime)
	rtMgr.SetNetworkLifecycle(
		func(ctx context.Context, slotIndex int, id, netns string) (string, error) {
			net, err := node.SetupSandboxNetwork(ctx, cfg.Network.BridgeName, slotIndex, id, netns)
			if err != nil {
				return "", err
			}
			return net.IP, nil
		},
		func(ctx context.Context, slotIndex int, id string) error {
			return node.TeardownSandboxNetwork(ctx, slotIndex, id)
		},
	)
	if err := rtMgr.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Failed to start runtime manager: %v\n", err)
		os.Exit(1)
	}
	defer rtMgr.Stop()

	log.Printf("[INFO] Node runtime initialized successfully. Host free disk: %d MB",
		rtMgr.DiskGuard().GetCurrentFreeMB())

	// 3. Initialize & Start Direct Terminal Gateway Server (:8443)
	gwSrv, err := node.NewContainerGatewayServer(cfg.Gateway, rtMgr)
	if err != nil {
		log.Printf("[WARN] Failed to configure Gateway Server: %v", err)
	} else {
		go func() {
			log.Printf("[INFO] Gateway Server listening on %s", cfg.Gateway.ListenAddr)
			if err := gwSrv.Start(); err != nil {
				log.Printf("[GATEWAY] Gateway server stopped: %v", err)
			}
		}()
		defer func() {
			_ = gwSrv.Stop(context.Background())
		}()
	}

	// 4. Initialize & Start Reverse Cluster Connection to Control Hub
	clusterClient := node.NewNodeControlClient(cfg, rtMgr, func(pubKey ed25519.PublicKey) {
		if gwSrv != nil {
			gwSrv.SetPublicKey(pubKey)
		}
	})
	clusterClient.Start(ctx)
	defer clusterClient.Stop()

	// Wait for OS termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[INFO] Shutting down node gracefully...")
}
