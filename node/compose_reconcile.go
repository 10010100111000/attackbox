package node

import (
	"context"
	"log"
	"time"
)

// One existing Worker lease cadence covers both runtimes, independently of the
// control connection. No per-container or per-interface timer is introduced.
func (c *NodeControlClient) experimentReaper(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var revision uint64
	dirty := true
	for {
		work, cancel := context.WithTimeout(ctx, 30*time.Second)
		if c.targets != nil {
			if err := c.targets.ReapLeases(work); err != nil && ctx.Err() == nil {
				log.Print("[TARGET] cleanup pending")
			}
		}
		if c.compose != nil {
			if err := c.compose.runtime.Reap(work); err != nil && ctx.Err() == nil {
				log.Print("[COMPOSE] cleanup pending")
			}
			if c.rtMgr != nil {
				current, _ := c.rtMgr.Inventory()
				if current != revision {
					dirty = true
					revision = current
				}
			}
			if dirty {
				report, err := c.compose.network.Reconcile(work)
				dirty = err != nil || len(report.Pending) > 0
				if dirty && ctx.Err() == nil {
					log.Print("[COMPOSE] network reconciliation pending")
				}
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
		}
	}
}
