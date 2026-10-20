package runtime

import (
	"context"
	"time"
)

// StartLocalReaper runs authoritative lifecycle reaper on Worker (TTL and Idle timeout).
// ReapSandboxes uses the same serialized stop path as explicit requests.
func (m *RuntimeManager) ReapSandboxes(ctx context.Context, now time.Time) {
	for _, sbx := range m.ListSnapshots() {
		if sbx.State == "creating" || sbx.State == "stopping" {
			continue
		}
		if sbx.CleanupError != "" || now.Unix() >= sbx.ExpiresAt || now.Sub(time.Unix(sbx.LastActiveAt, 0)) >= time.Duration(sbx.IdleSeconds)*time.Second {
			_ = m.DestroySandbox(ctx, sbx.ID)
		}
	}
}

func (m *RuntimeManager) StartLocalReaper(ctx context.Context) {
	ctx, m.reaperCancel = context.WithCancel(ctx)
	m.reaperWG.Add(1)
	go func() {
		defer m.reaperWG.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.ReapSandboxes(ctx, now)
			}
		}
	}()
}
