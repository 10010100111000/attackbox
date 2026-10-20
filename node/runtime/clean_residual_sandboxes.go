package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// CleanupResidualSandboxes implements restart-as-reset. No new work is accepted
// until all leftovers have been cleaned, including resources without a rootfs.
func (m *RuntimeManager) CleanupResidualSandboxes(ctx context.Context) error {
	ids := make(map[string]bool)
	for _, dir := range []string{m.cfg.InstancesDir, m.cfg.CgroupRoot} {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("scan residual resources: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				ids[e.Name()] = true
			}
		}
	}
	var errs []error
	for id := range ids {
		if !sandboxIDPattern.MatchString(id) {
			errs = append(errs, fmt.Errorf("invalid residual sandbox ID %q", id))
			continue
		}
		if err := m.DestroySandbox(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
