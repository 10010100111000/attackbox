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
	"time"

	"attackbox/internal/config"
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

func (m *Manager) Status(ctx context.Context, b *config.Box) Status {
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

// Start 派生 btrfs 瞬态快照并通过 Go 原生隔离逻辑拉起核心沙箱 (毫秒级)
func (m *Manager) Start(ctx context.Context, b *config.Box) error {
	_ = m.Kick(ctx, b)

	rootfs, err := m.Snapshots.Create(b.ID)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	if err := m.startCore(ctx, b, rootfs); err != nil {
		return fmt.Errorf("start core: %w", err)
	}

	return nil
}

// startCore 纯 Go 原生实现网络命名空间、veth 虚拟网卡、挂载设备树与权限初始化
func (m *Manager) startCore(ctx context.Context, b *config.Box, rootfs string) error {
	_ = os.MkdirAll(b.RunDir, 0777)
	_ = os.Chmod(b.RunDir, 01777)

	netns := "ab-" + b.ID
	vethHost := "veth-" + b.ID + "-br"
	vethGuest := "veth-" + b.ID
	boxIP := fmt.Sprintf("10.200.0.%d/24", 10+b.Display)

	// 1. 原生 ip netns 隔离网络环境
	out, _ := exec.CommandContext(ctx, "ip", "netns", "list").Output()
	if !strings.Contains(string(out), netns) {
		if err := exec.CommandContext(ctx, "ip", "netns", "add", netns).Run(); err != nil {
			return fmt.Errorf("ip netns add %s: %w", netns, err)
		}
	}

	// 2. 创建并配置 veth 虚拟网卡对
	if err := exec.CommandContext(ctx, "ip", "link", "show", vethHost).Run(); err != nil {
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethHost).Run()
		_ = exec.CommandContext(ctx, "ip", "link", "del", vethGuest).Run()
		_ = exec.CommandContext(ctx, "ip", "link", "add", vethGuest, "type", "veth", "peer", "name", vethHost).Run()
		_ = exec.CommandContext(ctx, "ip", "link", "set", vethHost, "master", "br0", "up").Run()
		_ = exec.CommandContext(ctx, "ip", "link", "set", vethGuest, "netns", netns).Run()

		_ = exec.CommandContext(ctx, "ip", "netns", "exec", netns, "ip", "link", "set", "lo", "up").Run()
		_ = exec.CommandContext(ctx, "ip", "netns", "exec", netns, "ip", "addr", "add", boxIP, "dev", vethGuest).Run()
		_ = exec.CommandContext(ctx, "ip", "netns", "exec", netns, "ip", "link", "set", vethGuest, "up").Run()
		_ = exec.CommandContext(ctx, "ip", "netns", "exec", netns, "ip", "route", "add", "default", "via", "10.200.0.1").Run()
	}

	// 3. 挂载环境与设备树准备
	dirs := []string{
		filepath.Join(rootfs, "run/attackbox"),
		filepath.Join(rootfs, "proc"),
		filepath.Join(rootfs, "sys"),
		filepath.Join(rootfs, "dev"),
		filepath.Join(rootfs, "dev/pts"),
		filepath.Join(rootfs, "dev/shm"),
		filepath.Join(rootfs, "tmp/.X11-unix"),
		filepath.Join(rootfs, "home"),
	}
	for _, d := range dirs {
		_ = os.MkdirAll(d, 0755)
	}

	_ = exec.CommandContext(ctx, "mount", "--bind", b.RunDir, filepath.Join(rootfs, "run/attackbox")).Run()
	_ = exec.CommandContext(ctx, "mount", "--rbind", "/dev", filepath.Join(rootfs, "dev")).Run()
	_ = exec.CommandContext(ctx, "mount", "--make-rprivate", filepath.Join(rootfs, "dev")).Run()
	_ = exec.CommandContext(ctx, "mount", "-t", "devpts", "devpts", filepath.Join(rootfs, "dev/pts"),
		"-o", "rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=666").Run()
	_ = exec.CommandContext(ctx, "mount", "-t", "tmpfs", "tmpfs", filepath.Join(rootfs, "dev/shm"),
		"-o", "rw,nosuid,nodev,mode=1777").Run()
	_ = exec.CommandContext(ctx, "chmod", "666", filepath.Join(rootfs, "dev/ptmx")).Run()

	_ = exec.CommandContext(ctx, "mount", "-t", "proc", "proc", filepath.Join(rootfs, "proc")).Run()
	_ = exec.CommandContext(ctx, "mount", "-t", "sysfs", "sys", filepath.Join(rootfs, "sys")).Run()
	_ = os.Chmod(filepath.Join(rootfs, "tmp"), 01777)
	_ = os.Chmod(filepath.Join(rootfs, "tmp/.X11-unix"), 01777)

	// 挂载独立 tmpfs 掩盖宿主上所有个人主目录，并仅构建沙箱渗透用户专用 /home/parrot
	_ = exec.CommandContext(ctx, "mount", "-t", "tmpfs", "tmpfs", filepath.Join(rootfs, "home"),
		"-o", "rw,nosuid,nodev,mode=0755").Run()

	parrotHome := filepath.Join(rootfs, "home/parrot")
	_ = os.MkdirAll(parrotHome, 0755)
	if _, err := os.Stat("/home/parrot"); err == nil {
		_ = exec.CommandContext(ctx, "cp", "-a", "/home/parrot/.", parrotHome+"/").Run()
	} else {
		_ = exec.CommandContext(ctx, "cp", "-r", "/etc/skel/.", parrotHome+"/").Run()
	}
	_ = exec.CommandContext(ctx, "chown", "-R", "parrot:parrot", parrotHome).Run()
	_ = os.Chmod(parrotHome, 0755)

	// 标记核心就绪
	_ = os.WriteFile(filepath.Join(b.RunDir, "core.ready"), []byte(time.Now().Format(time.RFC3339)), 0644)
	return nil
}

// Ensure 保证核心沙箱处于运行状态 (CLI 依赖)
func (m *Manager) Ensure(ctx context.Context, b *config.Box) error {
	if m.Status(ctx, b).Running() {
		return nil
	}
	return m.Start(ctx, b)
}

// EnsureGUI 保证图形桌面服务处于运行状态 (按需懒加载)
func (m *Manager) EnsureGUI(ctx context.Context, b *config.Box) error {
	if err := m.Ensure(ctx, b); err != nil {
		return err
	}

	guiSock := filepath.Join(b.RunDir, "gui.sock")
	if _, err := os.Stat(guiSock); err == nil {
		return nil
	}

	rootfs := m.Snapshots.PathFor(b.ID)
	return m.startGUI(ctx, b, rootfs)
}

// startGUI 纯 Go 原生调用拉起 TigerVNC 与 MATE 桌面
func (m *Manager) startGUI(ctx context.Context, b *config.Box, rootfs string) error {
	guiSock := filepath.Join(b.RunDir, "gui.sock")
	dispStr := fmt.Sprintf(":%d", b.Display)
	netns := "ab-" + b.ID

	_ = os.Remove(guiSock)
	_ = os.Remove(filepath.Join(rootfs, fmt.Sprintf("tmp/.X11-unix/X%d", b.Display)))
	_ = os.Remove(filepath.Join(rootfs, fmt.Sprintf("tmp/.X%d-lock", b.Display)))

	guiScript := fmt.Sprintf(`
/usr/bin/Xtigervnc "%s" -rfbunixpath /run/attackbox/gui.sock -rfbunixmode 0666 \
  -SecurityTypes None -AlwaysShared -geometry 1280x800 -depth 24 &
XVNC_PID=$!

for i in $(seq 1 40); do
  [ -S /tmp/.X11-unix/X%d ] && [ -S /run/attackbox/gui.sock ] && break
  sleep 0.1
done

su - parrot -c "
  export DISPLAY=%s
  export XDG_CURRENT_DESKTOP=MATE
  export XDG_SESSION_TYPE=x11
  gsettings set org.mate.Marco.general compositing-manager false 2>/dev/null || true
  gsettings set org.mate.interface enable-animations false 2>/dev/null || true
  exec dbus-run-session /usr/bin/mate-session
"
`, dispStr, b.Display, dispStr)

	cmd := exec.Command("ip", "netns", "exec", netns, "/usr/sbin/chroot", rootfs, "/bin/bash", "-c", guiScript)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gui: %w", err)
	}

	if cmd.Process != nil {
		_ = os.WriteFile(filepath.Join(b.RunDir, "gui.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	}

	// 轮询等待 gui.sock 真正创建就绪
	for i := 0; i < 40; i++ {
		if _, err := os.Stat(guiSock); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return fmt.Errorf("gui socket %s not ready after launch", guiSock)
}

// DetailedStatus 查询核心沙箱与图形桌面各自的运行状态
func (m *Manager) DetailedStatus(ctx context.Context, b *config.Box) (coreRunning bool, guiRunning bool) {
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
func (m *Manager) StopGUI(ctx context.Context, b *config.Box) error {
	guiPidFile := filepath.Join(b.RunDir, "gui.pid")
	if data, err := os.ReadFile(guiPidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-TERM", strconv.Itoa(pid)).Run()
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
		_ = os.Remove(guiPidFile)
	}

	_ = os.Remove(filepath.Join(b.RunDir, "gui.sock"))
	rootfs := m.Snapshots.PathFor(b.ID)
	_ = os.Remove(filepath.Join(rootfs, fmt.Sprintf("tmp/.X11-unix/X%d", b.Display)))
	_ = os.Remove(filepath.Join(rootfs, fmt.Sprintf("tmp/.X%d-lock", b.Display)))
	return nil
}

// Kick 终止沙箱全部进程、注销网络、卸载挂载并物理销毁快照
func (m *Manager) Kick(ctx context.Context, b *config.Box) error {
	// 1. 终止 GUI 进程
	_ = m.StopGUI(ctx, b)

	// 2. 终止 pid 进程（如存在）
	pidFile := filepath.Join(b.RunDir, "pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-TERM", strconv.Itoa(pid)).Run()
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
	}

	// 3. 杀掉沙箱渗透用户的孤儿进程
	_ = exec.CommandContext(ctx, "pkill", "-u", "parrot").Run()

	// 4. 清理网络 veth 与 netns
	netns := "ab-" + b.ID
	_ = exec.CommandContext(ctx, "ip", "netns", "del", netns).Run()
	_ = exec.CommandContext(ctx, "ip", "link", "del", "veth-"+b.ID+"-br").Run()

	// 5. 卸载挂载点
	rootfs := m.Snapshots.PathFor(b.ID)
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "run/attackbox")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "home")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "dev/pts")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "dev/shm")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "dev")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "proc")).Run()
	_ = exec.CommandContext(ctx, "umount", "-l", filepath.Join(rootfs, "sys")).Run()

	// 6. 物理销毁瞬态快照
	_ = m.Snapshots.Destroy(b.ID)

	// 7. 清理运行时文件
	_ = os.Remove(filepath.Join(b.RunDir, "core.ready"))
	_ = os.Remove(pidFile)

	return nil
}
