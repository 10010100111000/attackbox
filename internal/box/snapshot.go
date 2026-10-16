package box

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type SnapshotManager struct {
	BoxesDir   string
	BaseRootfs string
}

func NewSnapshotManager(boxesDir, baseRootfs string) *SnapshotManager {
	if boxesDir == "" {
		boxesDir = "/var/lib/attackbox/boxes"
	}
	if baseRootfs == "" {
		baseRootfs = "/var/lib/attackbox/base_rootfs"
	}
	return &SnapshotManager{
		BoxesDir:   boxesDir,
		BaseRootfs: baseRootfs,
	}
}

func (s *SnapshotManager) PathFor(slot string) string {
	return filepath.Join(s.BoxesDir, slot, "rootfs")
}

// EnsureBaseRootfs 检查基准子卷；若不存在则从 / 派生并立即深度脱敏后锁定为只读防篡改模板
func EnsureBaseRootfs(baseRootfs string) error {
	if baseRootfs == "" {
		baseRootfs = "/var/lib/attackbox/base_rootfs"
	}
	if _, err := os.Stat(baseRootfs); err == nil {
		return nil
	}

	log.Printf("[snapshot] base_rootfs not found at %s, initializing golden base...", baseRootfs)
	parent := filepath.Dir(baseRootfs)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create base_rootfs parent dir: %w", err)
	}

	// 1. 一次性从宿主机根目录派生基准子卷
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "/", baseRootfs)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create base snapshot: %v: %s", err, string(out))
	}

	// 2. 深度脱敏与清洗：移除宿主机所有私密数据与活动日志
	sensitivePaths := []string{
		filepath.Join(baseRootfs, "var/lib/attackbox"),
		filepath.Join(baseRootfs, "etc/attackbox"),
		filepath.Join(baseRootfs, "etc/ssl/novnc"),
	}
	for _, p := range sensitivePaths {
		_ = os.RemoveAll(p)
	}

	// 清理 root 与 home 下私密数据，清理临时文件与系统日志，并恢复标准 root 环境配置
	_ = exec.Command("sh", "-c", fmt.Sprintf("rm -rf %s/root/* %s/root/.* %s/home/* %s/tmp/* %s/var/tmp/* %s/var/log/* 2>/dev/null || true; cp -r /etc/skel/. %s/root/ 2>/dev/null || true",
		baseRootfs, baseRootfs, baseRootfs, baseRootfs, baseRootfs, baseRootfs, baseRootfs)).Run()

	// 确保沙箱必要目录骨架与权限
	dirs := []struct {
		path string
		perm os.FileMode
	}{
		{filepath.Join(baseRootfs, "run/attackbox"), 01777},
		{filepath.Join(baseRootfs, "tmp"), 01777},
		{filepath.Join(baseRootfs, "tmp/.X11-unix"), 01777},
		{filepath.Join(baseRootfs, "home"), 0755},
		{filepath.Join(baseRootfs, "root"), 0700},
	}
	for _, d := range dirs {
		_ = os.MkdirAll(d.path, d.perm)
		_ = os.Chmod(d.path, d.perm)
	}

	// 3. 锁定为只读防篡改模板
	_ = exec.Command("btrfs", "property", "set", baseRootfs, "ro", "true").Run()
	log.Printf("[snapshot] golden base_rootfs initialized and locked read-only at %s", baseRootfs)
	return nil
}

// Create 从只读基准子卷派生可写瞬态快照
func (s *SnapshotManager) Create(slot string) (string, error) {
	target := s.PathFor(slot)
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	// 确保旧快照已清理
	_ = s.Destroy(slot)

	// btrfs 从只读快照派生时，如果不加 -r，默认生成可读写快照
	cmd := exec.Command("btrfs", "subvolume", "snapshot", s.BaseRootfs, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("btrfs snapshot from base: %v: %s", err, string(out))
	}
	return target, nil
}

// Destroy 物理销毁快照
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
