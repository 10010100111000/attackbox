package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sandboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func (m *RuntimeManager) DestroySandbox(ctx context.Context, sandboxID string) error {
	if !sandboxIDPattern.MatchString(sandboxID) {
		return errors.New("invalid sandbox ID")
	}
	m.mu.Lock()
	rec, exists := m.sandboxes[sandboxID]
	if !exists {
		residual := false
		for _, path := range []string{filepath.Dir(m.RootfsPath(sandboxID)), m.CgroupPath(sandboxID)} {
			if _, err := os.Lstat(path); err == nil {
				residual = true
			} else if !os.IsNotExist(err) {
				m.mu.Unlock()
				return err
			}
		}
		if !residual {
			m.mu.Unlock()
			return nil
		}
		// An absent RAM record is not proof that disk/cgroup resources are gone.
		rec = &SandboxRecord{ID: sandboxID, State: "cleanup_failed"}
		m.sandboxes[sandboxID] = rec
	}
	m.mu.Unlock()
	rec.opMu.Lock()
	defer rec.opMu.Unlock()
	m.mu.RLock()
	current := m.sandboxes[sandboxID] == rec
	m.mu.RUnlock()
	if !current {
		return nil
	} // another stop already completed
	return m.destroyRecord(ctx, rec)
}

// destroyRecord requires rec.opMu. It is shared by rollback, retries and all stops.
func (m *RuntimeManager) destroyRecord(ctx context.Context, rec *SandboxRecord) error {
	m.mu.Lock()
	rec.State = "stopping"
	m.revision++
	rec.Revision = m.revision
	m.mu.Unlock()
	var errs []error
	if err := m.notifyLabGone(ctx, rec.ID); err != nil {
		errs = append(errs, err)
	}
	if rec.cmd != nil && rec.cmd.Process != nil {
		_ = rec.cmd.Process.Kill()
	}
	// cgroup.kill also handles processes surviving a daemon crash.
	if err := m.cgroups.Destroy(rec.ID); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if rec.waitDone != nil {
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-rec.waitDone:
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		case <-timer.C:
			errs = append(errs, errors.New("process exit timeout"))
		}
		timer.Stop()
	}
	if len(errs) == 0 {
		if err := m.unmountContainerFilesystems(ctx, m.RootfsPath(rec.ID)); err != nil {
			errs = append(errs, err)
		}
		if m.netTeardown != nil {
			if err := m.netTeardown(ctx, rec.SlotIndex, rec.ID); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			if err := m.btrfs.DestroySnapshot(ctx, rec.ID); err != nil {
				errs = append(errs, err)
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revision++
	if err := errors.Join(errs...); err != nil {
		rec.State = "cleanup_failed"
		rec.CleanupError = err.Error()
		rec.Revision = m.revision
		return fmt.Errorf("destroy sandbox %s: %w", rec.ID, err)
	}
	delete(m.sandboxes, rec.ID)
	return nil
}

func (m *RuntimeManager) unmountContainerFilesystems(ctx context.Context, rootfsPath string) error {
	mountPoints := []string{
		filepath.Join(rootfsPath, "dev/shm"),
		filepath.Join(rootfsPath, "dev/pts"),
		filepath.Join(rootfsPath, "dev"),
		filepath.Join(rootfsPath, "proc/sys"),
		filepath.Join(rootfsPath, "proc"),
		filepath.Join(rootfsPath, "sys"),
		filepath.Join(rootfsPath, "run"),
		filepath.Join(rootfsPath, "tmp"),
	}

	var errs []string
	for _, mp := range mountPoints {
		if _, err := os.Stat(mp); err == nil {
			out, err := exec.CommandContext(ctx, "umount", "-l", mp).CombinedOutput()
			if err != nil {
				if isMountPoint(mp) {
					errs = append(errs, fmt.Sprintf("umount %s: %v (%s)", mp, err, strings.TrimSpace(string(out))))
				}
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func isMountPoint(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}
