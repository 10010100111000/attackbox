package labcompose

import (
	"attackbox/node/labnetwork"
	"context"
	"fmt"
	"path/filepath"
)

// StopRequest also journals cancellation when admission or image validation
// rejected Start before creating an entry. Absence alone is never reported as
// stopped: Down must verify project ownership and actual resource absence.
func (m *Manager) StopRequest(ctx context.Context, req Request) (Status, error) {
	if err := m.enter(); err != nil {
		return Status{}, err
	}
	defer m.leave()
	if !idPattern.MatchString(req.ID) || !idPattern.MatchString(req.OwnerID) || req.ExpiresAt.IsZero() {
		return Status{}, fmt.Errorf("invalid stop identity")
	}
	d, err := m.loadDeployment(req.DeploymentID)
	if err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	e := m.entries[req.ID]
	if e == nil {
		r := Record{NodeID: m.cfg.NodeID, Request: req, Project: projectName(m.cfg.NodeID, req.ID), Networks: map[string]labnetwork.Network{}}
		r.VolumeNames = volumeNames(r.Project, d)
		if err := privateDir(filepath.Join(m.cfg.StateDir, "instances", req.ID)); err != nil {
			m.mu.Unlock()
			return Status{}, err
		}
		e = &entry{record: r}
		m.entries[req.ID] = e
		err = m.persist(e)
	} else {
		old := e.snapshot()
		if old.Request.ID != req.ID || old.Request.OwnerID != req.OwnerID || old.Request.DeploymentID != req.DeploymentID || !old.Request.ExpiresAt.Equal(req.ExpiresAt) {
			err = fmt.Errorf("stop identity mismatch")
		}
	}
	m.mu.Unlock()
	if err != nil {
		return Status{}, err
	}
	return m.Stop(ctx, req.ID)
}
