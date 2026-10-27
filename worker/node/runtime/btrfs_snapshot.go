package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BtrfsDriver manages btrfs subvolumes, snapshot derivations, and deletions.
type BtrfsDriver struct {
	BaseRootfs   string
	InstancesDir string
	imageBuilder *GoldenImageBuilder
}

// NewBtrfsDriver creates a new Btrfs snapshot manager.
func NewBtrfsDriver(baseRootfs, instancesDir string) *BtrfsDriver {
	return &BtrfsDriver{
		BaseRootfs:   baseRootfs,
		InstancesDir: instancesDir,
		imageBuilder: NewGoldenImageBuilder(baseRootfs),
	}
}

// EnsureBaseRootfs delegates to the unified GoldenImageBuilder pipeline.
func (b *BtrfsDriver) EnsureBaseRootfs() error {
	return b.imageBuilder.EnsureGoldenImage()
}

// CreateSnapshot derives a writable transient btrfs subvolume snapshot from the base image.
func (b *BtrfsDriver) CreateSnapshot(ctx context.Context, sandboxID string) (string, error) {
	targetDir := filepath.Join(b.InstancesDir, sandboxID, "rootfs")
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return "", fmt.Errorf("mkdir instance dir: %w", err)
	}

	// Ensure golden base image is ready and sanitized
	if err := b.EnsureBaseRootfs(); err != nil {
		return "", fmt.Errorf("ensure base rootfs: %w", err)
	}

	// Execute atomic btrfs subvolume snapshot
	cmd := exec.CommandContext(ctx, "btrfs", "subvolume", "snapshot", b.BaseRootfs, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("btrfs snapshot: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return targetDir, nil
}

// DestroySnapshot atomically deletes a transient instance btrfs subvolume snapshot.
func (b *BtrfsDriver) DestroySnapshot(ctx context.Context, sandboxID string) error {
	targetDir := filepath.Join(b.InstancesDir, sandboxID, "rootfs")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		err := os.Remove(filepath.Dir(targetDir))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cmd := exec.CommandContext(ctx, "btrfs", "subvolume", "delete", targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return os.Remove(filepath.Join(b.InstancesDir, sandboxID))
}
