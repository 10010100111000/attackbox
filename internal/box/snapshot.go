package box

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type SnapshotManager struct {
	BaseDir string
}

func NewSnapshotManager(baseDir string) *SnapshotManager {
	return &SnapshotManager{BaseDir: baseDir}
}

func (s *SnapshotManager) PathFor(slot string) string {
	return filepath.Join(s.BaseDir, slot, "rootfs")
}

func (s *SnapshotManager) Create(slot string) (string, error) {
	target := s.PathFor(slot)
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	// 确保旧快照已清理
	_ = s.Destroy(slot)

	cmd := exec.Command("btrfs", "subvolume", "snapshot", "/", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("btrfs snapshot: %v: %s", err, string(out))
	}
	return target, nil
}

func (s *SnapshotManager) Destroy(slot string) error {
	target := s.PathFor(slot)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}
	cmd := exec.Command("btrfs", "subvolume", "delete", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs delete: %v: %s", err, string(out))
	}
	return nil
}
