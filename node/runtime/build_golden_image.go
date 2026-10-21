package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GoldenImageBuilder manages automated snapshotting and deep sanitization of the golden base rootfs.
type GoldenImageBuilder struct {
	targetDir string
}

// NewGoldenImageBuilder constructs a builder for the golden rootfs image.
func NewGoldenImageBuilder(targetDir string) *GoldenImageBuilder {
	return &GoldenImageBuilder{
		targetDir: targetDir,
	}
}

// EnsureGoldenImage validates that the golden base exists, is complete, and is locked read-only.
// If it does not exist or is incomplete/unlocked, it executes the full derive-sanitize-lock pipeline from host '/'
// using an isolated temporary subvolume with atomic promotion.
func (b *GoldenImageBuilder) EnsureGoldenImage() error {
	if b.isGoldenImageHealthy(b.targetDir) {
		return nil
	}

	// If directory exists but failed health checks, clean it up
	if _, err := os.Stat(b.targetDir); err == nil {
		fmt.Printf("[WARN] Existing golden image at %s is corrupted, incomplete, or not read-only. Rebuilding...\n", b.targetDir)
		_ = b.cleanupSubvolume(b.targetDir)
	}

	if err := os.MkdirAll(filepath.Dir(b.targetDir), 0755); err != nil {
		return fmt.Errorf("create golden image parent dir: %w", err)
	}

	tmpTarget := fmt.Sprintf("%s.building.%d", b.targetDir, time.Now().UnixNano())

	// 1. Take snapshot of host root '/' into isolated temporary subvolume
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "/", tmpTarget)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs snapshot / to %s failed: %s (%w)", tmpTarget, strings.TrimSpace(string(out)), err)
	}

	// 2. Unlock as writable temporarily for in-place sanitization
	if err := setSubvolumeReadOnly(tmpTarget, false); err != nil {
		_ = b.cleanupSubvolume(tmpTarget)
		return fmt.Errorf("unlock building golden image: %w", err)
	}

	// 3. Deep sanitize rootfs (configs, secrets, credentials, users, shadow, passwd, logs)
	if err := b.SanitizeRootfs(tmpTarget); err != nil {
		_ = b.cleanupSubvolume(tmpTarget)
		return fmt.Errorf("sanitize golden image: %w", err)
	}

	// 4. Write completion marker
	markerPath := filepath.Join(tmpTarget, "etc/.attackbox_golden_ready")
	if err := os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0444); err != nil {
		_ = b.cleanupSubvolume(tmpTarget)
		return fmt.Errorf("write completion marker: %w", err)
	}

	// 5. Lock image as physical read-only for tamper resistance
	if err := setSubvolumeReadOnly(tmpTarget, true); err != nil {
		_ = b.cleanupSubvolume(tmpTarget)
		return fmt.Errorf("lock golden image read-only: %w", err)
	}

	// 6. Verify read-only and integrity before atomic promotion
	if !b.isGoldenImageHealthy(tmpTarget) {
		_ = b.cleanupSubvolume(tmpTarget)
		return fmt.Errorf("golden image health verification failed after build")
	}

	// 7. Atomic promotion via rename
	if err := os.Rename(tmpTarget, b.targetDir); err != nil {
		_ = b.cleanupSubvolume(tmpTarget)
		return fmt.Errorf("atomic promote golden image to %s: %w", b.targetDir, err)
	}

	fmt.Printf("[INFO] Successfully built and atomically locked golden rootfs at %s\n", b.targetDir)
	return nil
}

// SnapshotHostRoot derives an initial btrfs subvolume snapshot from the host '/'.
func (b *GoldenImageBuilder) SnapshotHostRoot() error {
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "/", b.targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs snapshot / to %s failed: %s (%w)", b.targetDir, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (b *GoldenImageBuilder) isGoldenImageHealthy(subvolPath string) bool {
	if _, err := os.Stat(subvolPath); err != nil {
		return false
	}

	// 1. Verify read-only property is strictly true
	cmd := exec.Command("btrfs", "property", "get", subvolPath, "ro")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ro=true") {
		return false
	}

	// 2. Verify completion marker exists
	markerPath := filepath.Join(subvolPath, "etc/.attackbox_golden_ready")
	if _, err := os.Stat(markerPath); err != nil {
		return false
	}

	return true
}

func setSubvolumeReadOnly(subvolPath string, ro bool) error {
	roVal := "false"
	if ro {
		roVal = "true"
	}
	cmd := exec.Command("btrfs", "property", "set", subvolPath, "ro", roVal)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs set ro %s: %s (%w)", roVal, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (b *GoldenImageBuilder) cleanupSubvolume(subvolPath string) error {
	cmd := exec.Command("btrfs", "subvolume", "delete", subvolPath)
	_ = cmd.Run()
	return os.RemoveAll(subvolPath)
}

func (b *GoldenImageBuilder) unlockImageWritable() error {
	return setSubvolumeReadOnly(b.targetDir, false)
}

func (b *GoldenImageBuilder) lockImageReadOnly() error {
	return setSubvolumeReadOnly(b.targetDir, true)
}

func (b *GoldenImageBuilder) cleanupFailedImage() error {
	return b.cleanupSubvolume(b.targetDir)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}
