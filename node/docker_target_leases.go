package node

import (
	"context"
	"log"
	"time"
)

func (m *DockerTargetManager) now() time.Time {
	if m.clock != nil {
		return m.clock()
	}
	return time.Now()
}
func (m *DockerTargetManager) expired(r targetRecord) bool {
	// Legacy records without a deadline are reclaimed, never granted infinity.
	return r.Request.ExpiresAt.IsZero() || !r.Request.ExpiresAt.After(m.now())
}

// ReapLeases runs independently of the control connection and browser activity.
// Persisting the stop tombstone before removal prevents delayed starts from
// reviving an expired ID. Failed removals remain obligations for the next sweep.
func (m *DockerTargetManager) ReapLeases(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var first error
	for _, r := range m.records {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if r.Cleaned || (r.Desired != "stopped" && !m.expired(r)) {
			continue
		}
		if _, err := m.stopRecord(ctx, r); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func (m *DockerTargetManager) RunLeaseReaper(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := m.ReapLeases(sweepCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("[TARGET] lease cleanup pending: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
