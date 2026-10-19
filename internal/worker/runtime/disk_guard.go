package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"attackbox/internal/worker/config"
)

// DiskGuard monitors the host btrfs filesystem free space to prevent disk exhaustion.
type DiskGuard struct {
	cfg          config.DiskGuardConfig
	rootPath     string
	tripped      atomic.Bool
	warned       atomic.Bool
	currentFree  atomic.Int64
	stopChan     chan struct{}
	wg           sync.WaitGroup
	onFuseAction func(freeMB int64)
}

// NewDiskGuard creates a disk watermark monitor.
func NewDiskGuard(cfg config.DiskGuardConfig, rootPath string, onFuse func(freeMB int64)) *DiskGuard {
	if cfg.CheckIntervalSec <= 0 {
		cfg.CheckIntervalSec = 3
	}
	if cfg.MinFreeSpaceMB <= 0 {
		cfg.MinFreeSpaceMB = 5120 // 5 GB
	}
	if cfg.WarnFreeSpaceMB <= 0 {
		cfg.WarnFreeSpaceMB = 10240 // 10 GB
	}
	if rootPath == "" {
		rootPath = "/"
	}

	return &DiskGuard{
		cfg:          cfg,
		rootPath:     rootPath,
		stopChan:     make(chan struct{}),
		onFuseAction: onFuse,
	}
}

// IsTripped returns true if the host disk capacity has fallen below the safety floor.
func (d *DiskGuard) IsTripped() bool {
	return d.tripped.Load()
}

// IsWarning returns true if host disk capacity is in the warning band.
func (d *DiskGuard) IsWarning() bool {
	return d.warned.Load()
}

// GetCurrentFreeMB returns the last recorded free space in megabytes.
func (d *DiskGuard) GetCurrentFreeMB() int64 {
	return d.currentFree.Load()
}

// Start begins periodic disk polling in a background goroutine.
func (d *DiskGuard) Start(ctx context.Context) {
	if !d.cfg.Enabled {
		return
	}

	// Immediate synchronous probe so current capacity is available right away
	d.check()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(time.Duration(d.cfg.CheckIntervalSec) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-d.stopChan:
				return
			case <-ticker.C:
				d.check()
			}
		}
	}()
}

// Stop cleanly terminates the disk monitor.
func (d *DiskGuard) Stop() {
	select {
	case <-d.stopChan:
	default:
		close(d.stopChan)
	}
	d.wg.Wait()
}

func (d *DiskGuard) check() {
	freeMB, err := getFileSystemFreeMB(d.rootPath)
	if err != nil {
		fmt.Printf("[WARN] DiskGuard statfs failed on %s: %v\n", d.rootPath, err)
		return
	}
	d.currentFree.Store(freeMB)

	// Check trip condition (hard fuse)
	if freeMB < d.cfg.MinFreeSpaceMB {
		if !d.tripped.Swap(true) {
			fmt.Printf("[CRITICAL] DiskGuard tripped: free space (%d MB) < threshold (%d MB). Fusing new sandboxes.\n",
				freeMB, d.cfg.MinFreeSpaceMB)
			if d.onFuseAction != nil {
				d.onFuseAction(freeMB)
			}
		}
	} else {
		// Recovery
		if d.tripped.Swap(false) {
			fmt.Printf("[INFO] DiskGuard recovered: free space (%d MB) >= threshold (%d MB).\n",
				freeMB, d.cfg.MinFreeSpaceMB)
		}
	}

	// Check warning condition
	d.warned.Store(freeMB < d.cfg.WarnFreeSpaceMB)
}
