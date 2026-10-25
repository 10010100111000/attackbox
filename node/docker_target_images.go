package node

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Starting a learner instance never downloads images.
func (m *DockerTargetManager) requireImage(ctx context.Context, image string) error {
	code, err := dockerNetworkRequest(ctx, m.client, "GET", "/images/"+url.PathEscape(image)+"/json", nil, nil)
	if code == 404 {
		return fmt.Errorf("target_image_not_prepared")
	}
	if err != nil {
		return fmt.Errorf("image_inspect_failed")
	}
	return nil
}

// Preparation is independent of the instance queue and lifecycle mutex. Only
// one download runs on a node, with duplicate requests sharing its status.
// After restart, Docker's actual image inventory remains the source of truth.
func (m *DockerTargetManager) ImageStatus(ctx context.Context, image string, prepare bool) (string, error) {
	m.imageMu.Lock()
	defer m.imageMu.Unlock()
	if m.imageJobs == nil {
		m.imageJobs = make(map[string]string)
	}
	if m.imageJobs[image] == "preparing" {
		return "preparing", nil
	}
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := m.requireImage(inspectCtx, image)
	if err == nil {
		delete(m.imageJobs, image)
		return "ready", nil
	}
	if err.Error() != "target_image_not_prepared" {
		return "", err
	}
	if !prepare {
		if m.imageJobs[image] == "failed" {
			return "failed", nil
		}
		return "missing", nil
	}
	for _, state := range m.imageJobs {
		if state == "preparing" {
			return "", fmt.Errorf("image_preparation_busy")
		}
	}
	m.imageJobs[image] = "preparing"
	go func() {
		downloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		err := m.ensureImage(downloadCtx, image)
		m.imageMu.Lock()
		defer m.imageMu.Unlock()
		if err != nil {
			m.imageJobs[image] = "failed"
		} else {
			delete(m.imageJobs, image)
		}
	}()
	return "preparing", nil
}
