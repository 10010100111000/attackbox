// Package box owns the lifecycle of the per-user X sessions and btrfs ephemeral snapshots.
package box

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"attackbox/internal/cfg"
)

type Manager struct {
	Snapshots *SnapshotManager
}

func NewManager() *Manager {
	return &Manager{
		Snapshots: NewSnapshotManager("/var/lib/attackbox/boxes"),
	}
}

// Status is what the system currently thinks about a box session.
type Status struct {
	ActiveState string
	SubState    string
	MainPID     int
}

func (s Status) Running() bool { return s.ActiveState == "active" }

func (m *Manager) Status(ctx context.Context, b *cfg.Box) Status {
	// 检查沙箱核心就绪标记及网络命名空间是否存在
	coreFile := filepath.Join(b.RunDir, "core.ready")
	if _, err := os.Stat(coreFile); err == nil {
		netns := "ab-" + b.ID
		if out, err := exec.CommandContext(ctx, "ip", "netns", "list").Output(); err == nil {
			if strings.Contains(string(out), netns) {
				return Status{
					ActiveState: "active",
					SubState:    "running",
				}
			}
		}
	}

	// 兼容检查 RunDir/pid 文件
	pidFile := filepath.Join(b.RunDir, "pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			if err := exec.CommandContext(ctx, "kill", "-0", strconv.Itoa(pid)).Run(); err == nil {
				return Status{
					ActiveState: "active",
					SubState:    "running",
					MainPID:     pid,
				}
			}
		}
	}

	return Status{ActiveState: "inactive", SubState: "dead"}
}

// Start 派生 btrfs 瞬态快照并通过引导脚本拉起核心沙箱 (毫秒级)
func (m *Manager) Start(ctx context.Context, b *cfg.Box) error {
	// 1. 确保旧环境清理干净
	_ = m.Kick(ctx, b)

	// 2. 派生瞬态快照
	rootfs, err := m.Snapshots.Create(b.ID)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	// 3. 运行核心沙箱引导 (core 模式)
	ipSuffix := strconv.Itoa(10 + b.Display)
	dispStr := strconv.Itoa(b.Display)

	cmd := exec.CommandContext(ctx, b.SessionScript, b.ID, ipSuffix, dispStr, rootfs, b.RunDir, "core")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launch box core: %v: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// Ensure 保证核心沙箱处于运行状态 (CLI 依赖)
func (m *Manager) Ensure(ctx context.Context, b *cfg.Box) error {
	if m.Status(ctx, b).Running() {
		return nil
	}
	return m.Start(ctx, b)
}

// EnsureGUI 保证图形桌面服务处于运行状态 (按需懒加载)
func (m *Manager) EnsureGUI(ctx context.Context, b *cfg.Box) error {
	if err := m.Ensure(ctx, b); err != nil {
		return err
	}

	// 检查 gui.sock 是否已经就绪
	guiSock := filepath.Join(b.RunDir, "gui.sock")
	if _, err := os.Stat(guiSock); err == nil {
		return nil
	}

	rootfs := m.Snapshots.PathFor(b.ID)
	ipSuffix := strconv.Itoa(10 + b.Display)
	dispStr := strconv.Itoa(b.Display)

	cmd := exec.CommandContext(ctx, b.SessionScript, b.ID, ipSuffix, dispStr, rootfs, b.RunDir, "gui")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launch box gui: %v: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// DetailedStatus 查询核心沙箱与图形桌面各自的运行状态
func (m *Manager) DetailedStatus(ctx context.Context, b *cfg.Box) (coreRunning bool, guiRunning bool) {
	coreRunning = m.Status(ctx, b).Running()
	if coreRunning {
		guiSock := filepath.Join(b.RunDir, "gui.sock")
		if _, err := os.Stat(guiSock); err == nil {
			guiRunning = true
		}
	}
	return coreRunning, guiRunning
}

// StopGUI 仅终止 TigerVNC 与 MATE 桌面，保持核心沙箱（CLI）继续存活
func (m *Manager) StopGUI(ctx context.Context, b *cfg.Box) error {
	rootfs := m.Snapshots.PathFor(b.ID)
	ipSuffix := strconv.Itoa(10 + b.Display)
	dispStr := strconv.Itoa(b.Display)

	cmd := exec.CommandContext(ctx, b.SessionScript, b.ID, ipSuffix, dispStr, rootfs, b.RunDir, "stop-gui")
	_ = cmd.Run()

	_ = os.Remove(filepath.Join(b.RunDir, "gui.sock"))
	_ = os.Remove(filepath.Join(b.RunDir, "gui.pid"))
	return nil
}

// Kick 终止沙箱全部进程、注销网络、卸载挂载并物理销毁快照
func (m *Manager) Kick(ctx context.Context, b *cfg.Box) error {
	// 1. 终止 GUI 进程
	guiPidFile := filepath.Join(b.RunDir, "gui.pid")
	if data, err := os.ReadFile(guiPidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-TERM", strconv.Itoa(pid)).Run()
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
	}

	// 2. 终止 pid 进程（如存在）
	pidFile := filepath.Join(b.RunDir, "pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-TERM", strconv.Itoa(pid)).Run()
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
	}

	// 3. 杀掉宿主上相关用户的孤儿进程
	if b.User != "" {
		_ = exec.CommandContext(ctx, "pkill", "-u", b.User).Run()
	}

	// 4. 清理网络 veth 与 netns
	netns := "ab-" + b.ID
	_ = exec.CommandContext(ctx, "ip", "netns", "del", netns).Run()
	_ = exec.CommandContext(ctx, "ip", "link", "del", "veth-"+b.ID+"-br").Run()

	// 5. 卸载挂载点
	rootfs := m.Snapshots.PathFor(b.ID)
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "run/attackbox")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "home/laji")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "dev/pts")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "dev/shm")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "dev")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "proc")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "sys")).Run()

	// 6. 物理销毁瞬态快照
	_ = m.Snapshots.Destroy(b.ID)

	// 7. 清理运行时文件
	_ = os.Remove(filepath.Join(b.RunDir, "gui.sock"))
	_ = os.Remove(filepath.Join(b.RunDir, "tmux.sock"))
	_ = os.Remove(filepath.Join(b.RunDir, "core.ready"))
	_ = os.Remove(guiPidFile)
	_ = os.Remove(pidFile)

	return nil
}
