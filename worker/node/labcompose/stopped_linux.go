//go:build linux

package labcompose

import (
	"context"

	"attackbox/node/labnetwork"
)

// A stopped record is a lease-bounded rejection marker, not a resource archive.
// Keep network acknowledgements in the record until each has succeeded; absence
// on retry is safe because ForgetNetwork only follows a durable Cleaned commit.
// The caller holds e.op.
func (m *Manager) finishStopped(ctx context.Context, e *entry) (Status, error) {
	r := e.snapshot()
	for _, n := range r.Networks {
		if err := m.cfg.Networks.ForgetNetwork(ctx, n.ID); err != nil {
			return pending(r), err
		}
	}
	if len(r.Networks) > 0 || len(r.VolumeNames) > 0 || r.StartIssued {
		e.mu.Lock()
		next := Record{NodeID: r.NodeID, Request: r.Request, Project: r.Project, Cleaned: true, Networks: map[string]labnetwork.Network{}}
		err := m.commitRecord(e, next)
		e.mu.Unlock()
		if err != nil {
			return pending(r), err
		}
		r = e.snapshot()
	}
	return m.status(ctx, r, Deployment{})
}
