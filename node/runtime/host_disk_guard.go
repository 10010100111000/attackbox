package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// DiskGuard monitors the host btrfs filesystem free space to prevent disk exhaustion.
type DiskGuard struct {
	cfg          DiskGuardConfig
	rootPath     string
	tripped      atomic.Bool
	warned       atomic.Bool
	currentFree  atomic.Int64
	stopChan     chan struct{}
	wg           sync.WaitGroup
	onFuseAction func(freeMB int64)
}

// NewDiskGuard creates a disk watermark monitor.
func NewDiskGuard(cfg DiskGuardConfig, rootPath string, onFuse func(freeMB int64)) *DiskGuard {
	if cfg.CheckIntervalSec <= 0 {
		cfg.CheckIntervalSec = 3
	}
	if cfg.MinFreeSpaceMB <= 0 {
		cfg.MinFreeSpaceMB = 5120 // 5GB safety threshold
	}
	if cfg.WarnFreeSpaceMB <= 0 {
		cfg.WarnFreeSpaceMB = 10240 // 10GB warning threshold
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

// Start begins periodic polling of filesystem statistics.
func (g *DiskGuard) Start(ctx context.Context) {
	if !g.cfg.Enabled {
		return
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.runLoop(ctx)
	}()
}

// Stop cleanly terminates the monitor loop.
func (g *DiskGuard) Stop() {
	if !g.cfg.Enabled {
		return
	}
	select {
	case <-g.stopChan:
	default:
		close(g.stopChan)
	}
	g.wg.Wait()
}

// IsTripped returns true if disk space has breached the critical threshold.
func (g *DiskGuard) IsTripped() bool {
	if !g.cfg.Enabled {
		return false
	}
	return g.tripped.Load()
}

// GetCurrentFreeMB returns the latest sampled free disk space in megabytes.
func (g *DiskGuard) GetCurrentFreeMB() int64 {
	return g.currentFree.Load()
}

func (g *DiskGuard) runLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(g.cfg.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	// Perform immediate first check
	g.checkWatermark()

	for {
		select {
		case <-ctx.Done():
			return
		case <-g.stopChan:
			return
		case <-ticker.C:
			g.checkWatermark()
		}
	}
}

func (g *DiskGuard) checkWatermark() {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(g.rootPath, &stat); err != nil {
		return
	}

	freeBytes := stat.Bavail * uint64(stat.Bsize)
	freeMB := int64(freeBytes / (1024 * 1024))
	g.currentFree.Store(freeMB)

	// Critical threshold breached -> Trip circuit breaker
	if freeMB <= g.cfg.MinFreeSpaceMB {
		if !g.tripped.Swap(true) {
			if g.onFuseAction != nil {
				g.onFuseAction(freeMB)
			}
		}
		return
	}

	// Warning threshold breached
	if freeMB <= g.cfg.WarnFreeSpaceMB {
		if !g.warned.Swap(true) {
			fmt.Printf("[WARN] Host filesystem low disk space warning: %d MB remaining.\n", freeMB)
		}
	} else {
		g.warned.Store(false)
	}

	// Recovery check
	if g.tripped.Load() && freeMB > g.cfg.MinFreeSpaceMB+1024 {
		g.tripped.Store(false)
		fmt.Printf("[INFO] Host disk circuit breaker reset. Free space restored to %d MB.\n", freeMB)
	}
}
