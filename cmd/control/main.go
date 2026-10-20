package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"attackbox/control"
)

func main() {
	configPath := flag.String("config", "configs/control.yaml", "Path to Control Hub configuration file")
	flag.Parse()

	log.Println("[BOOT] Starting AttackBox Control Hub...")

	// 1. Load configuration from YAML
	cfg, err := control.LoadControlConfig(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Load configuration: %v", err)
	}

	// 2. Load or generate Ed25519 Keys
	pubKey, privKey, err := control.LoadOrGenerateKeys(cfg.Auth.Ed25519PrivateKey)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize Ed25519 keys: %v", err)
	}
	log.Printf("[AUTH] Ed25519 Public Key: %s", hex.EncodeToString(pubKey))

	// 3. Initialize SQLite DB
	db, err := control.NewDatabaseStore(cfg.Database.Path)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("[STORE] SQLite database initialized at %s", cfg.Database.Path)

	// 4. Initialize Node Cluster Manager
	clusterMgr := control.NewNodeClusterManager(cfg.Cluster.ClusterSecret)
	clusterMgr.SetTimeouts(cfg.Cluster.HeartbeatTimeoutSec, cfg.Cluster.RPCTimeoutSec)
	clusterMgr.SetPublicKey(pubKey)

	// 5. Initialize Control HTTP Server
	router := control.NewControlHTTPServer(cfg, db, privKey, pubKey, clusterMgr, nil, nil)

	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// 6. Start HTTP listener in background
	go func() {
		log.Printf("[HTTP] Control Hub listening on http://%s", cfg.Server.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// 7. Wait for OS termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[SHUTDOWN] Shutting down Control Hub gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[WARN] HTTP server shutdown error: %v", err)
	}
	fmt.Println("[SHUTDOWN] Control Hub stopped.")
}
