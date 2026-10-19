package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BtrfsDriver manages base rootfs gold images and transient snapshot subvolumes.
type BtrfsDriver struct {
	BaseRootfs   string
	InstancesDir string
}

// NewBtrfsDriver creates a new Btrfs snapshot manager.
func NewBtrfsDriver(baseRootfs, instancesDir string) *BtrfsDriver {
	return &BtrfsDriver{
		BaseRootfs:   baseRootfs,
		InstancesDir: instancesDir,
	}
}

// EnsureBaseRootfs verifies whether the golden base subvolume exists.
// If it does not exist, it clones from host '/' and sanitizes host-specific credentials.
func (b *BtrfsDriver) EnsureBaseRootfs() error {
	if _, err := os.Stat(b.BaseRootfs); err == nil {
		// Base rootfs already exists
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(b.BaseRootfs), 0755); err != nil {
		return fmt.Errorf("create base rootfs parent dir: %w", err)
	}

	// 1. Take snapshot of host root '/'
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "/", b.BaseRootfs)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs snapshot / to %s: %s (%w)", b.BaseRootfs, strings.TrimSpace(string(out)), err)
	}

	// 2. Deep sanitize golden image
	b.sanitizeBase(b.BaseRootfs)

	// 3. Mark read-only for tamper resistance
	roCmd := exec.Command("btrfs", "property", "set", b.BaseRootfs, "ro", "true")
	if out, err := roCmd.CombinedOutput(); err != nil {
		// Log warning but do not hard abort if property set fails
		fmt.Printf("[WARN] Failed to set base rootfs read-only: %s (%v)\n", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// sanitizeBase cleans host credentials, logs, and sensitive data inside the template.
func (b *BtrfsDriver) sanitizeBase(dir string) {
	// Sensitive paths to clear
	wipePatterns := []string{
		filepath.Join(dir, "etc/ssh/ssh_host_*"),
		filepath.Join(dir, "root/.ssh/*"),
		filepath.Join(dir, "home/*/.ssh/*"),
		filepath.Join(dir, "tmp/*"),
		filepath.Join(dir, "var/tmp/*"),
		filepath.Join(dir, "var/log/*"),
		filepath.Join(dir, "var/run/*"),
	}

	for _, pattern := range wipePatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, m := range matches {
				_ = os.RemoveAll(m)
			}
		}
	}

	// Ensure critical virtual directories exist
	dirs := []string{
		filepath.Join(dir, "proc"),
		filepath.Join(dir, "sys"),
		filepath.Join(dir, "dev"),
		filepath.Join(dir, "tmp"),
		filepath.Join(dir, "run"),
		filepath.Join(dir, "etc"),
	}
	for _, d := range dirs {
		_ = os.MkdirAll(d, 0755)
	}
	_ = os.Chmod(filepath.Join(dir, "tmp"), 01777)
}

// CreateSnapshot derives a writable transient btrfs subvolume snapshot from the base image.
func (b *BtrfsDriver) CreateSnapshot(sandboxID string) (string, error) {
	targetDir := filepath.Join(b.InstancesDir, sandboxID, "rootfs")
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return "", fmt.Errorf("mkdir instance dir: %w", err)
	}

	// Ensure base image is ready
	if err := b.EnsureBaseRootfs(); err != nil {
		return "", fmt.Errorf("ensure base rootfs: %w", err)
	}

	cmd := exec.Command("btrfs", "subvolume", "snapshot", b.BaseRootfs, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("btrfs snapshot creation failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return targetDir, nil
}

// DestroySnapshot cleanly deletes the sandbox's btrfs subvolume snapshot.
func (b *BtrfsDriver) DestroySnapshot(sandboxID string) error {
	targetDir := filepath.Join(b.InstancesDir, sandboxID, "rootfs")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		// Nothing to clean
		return nil
	}

	cmd := exec.Command("btrfs", "subvolume", "delete", targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete %s failed: %s (%w)", targetDir, strings.TrimSpace(string(out)), err)
	}

	// Remove instance parent dir
	_ = os.RemoveAll(filepath.Join(b.InstancesDir, sandboxID))
	return nil
}
