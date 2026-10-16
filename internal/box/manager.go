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
	"sync"
	"time"

	"attackbox/internal/config"
)

type Manager struct {
	Snapshots *SnapshotManager
	boxMu     sync.Map // boxID string -> *sync.Mutex
}

func NewManager() *Manager {
	return NewManagerWithConfig("/var/lib/attackbox/boxes", "/var/lib/attackbox/base_rootfs")
}

func NewManagerWithConfig(boxesDir, baseRootfs string) *Manager {
	return &Manager{
		Snapshots: NewSnapshotManager(boxesDir, baseRootfs),
	}
}

// boxLock 获取针对指定沙箱槽位的细粒度互斥锁
func (m *Manager) boxLock(boxID string) *sync.Mutex {
	v, _ := m.boxMu.LoadOrStore(boxID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// findChildPID 查找 unshare 在宿主机的直接子进程 PID (即沙箱内部的 PID 1)
func findChildPID(parentPID int) (int, error) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(parentPID)).Output()
	if err == nil {
		pids := strings.Fields(string(out))
		if len(pids) > 0 {
			if pid, err := strconv.Atoi(pids[0]); err == nil && pid > 0 {
				return pid, nil
			}
		}
	}
	// 备选读取 /proc/<parentPID>/task/<parentPID>/children
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", parentPID, parentPID)); err == nil {
		pids := strings.Fields(string(data))
		if len(pids) > 0 {
			if pid, err := strconv.Atoi(pids[0]); err == nil && pid > 0 {
				return pid, nil
			}
		}
	}
	return 0, fmt.Errorf("child of %d not found", parentPID)
}

// Status is what the system currently thinks about a box session.
type Status struct {
	ActiveState string
	SubState    string
	MainPID     int
}

func (s Status) Running() bool { return s.ActiveState == "active" }

func (m *Manager) Status(ctx context.Context, b *config.Box) Status {
	initPidFile := filepath.Join(b.RunDir, "init.pid")
	if data, err := os.ReadFile(initPidFile); err == nil {
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

// Start 从只读基准子卷派生瞬态快照并拉起全命名空间常驻 Init (线程安全)
func (m *Manager) Start(ctx context.Context, b *config.Box) error {
	mu := m.boxLock(b.ID)
	mu.Lock()
	defer mu.Unlock()
	return m.startLocked(ctx, b)
}

func (m *Manager) startLocked(ctx context.Context, b *config.Box) error {
	_ = m.kickLocked(ctx, b)

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
	_ = os.MkdirAll(b.RunDir, 01777)
	_ = os.Chmod(b.RunDir, 01777)

	// 清理历史残留 PID 与状态文件
	_ = os.Remove(filepath.Join(b.RunDir, "init.pid"))
	_ = os.Remove(filepath.Join(b.RunDir, "unshare.pid"))
	_ = os.Remove(filepath.Join(b.RunDir, "core.ready"))

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

	// 3. 构造私有 Mount/PID Namespace 内执行的挂载与 Init 脚本
	initScript := fmt.Sprintf(`
# 挂载命名空间私有化，杜绝向宿主机扩散
mount --make-rprivate /

# 挂载设备树与独立虚拟文件系统
mount --bind "%s" "%s"
mount --make-rprivate "%s"
mount --rbind /dev "%s/dev"
mount --make-rprivate "%s/dev"
mount -t devpts devpts "%s/dev/pts" -o rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=666
mount -t tmpfs tmpfs "%s/dev/shm" -o rw,nosuid,nodev,mode=1777
chmod 666 "%s/dev/ptmx" 2>/dev/null || true

# 挂载属于本沙箱独立 PID namespace 的 proc 与 sys
mount -t proc proc "%s/proc"
mount -t sysfs sys "%s/sys"

# 挂载宿主-沙箱通信套接字目录
mkdir -p "%s/run/attackbox"
mount --bind "%s" "%s/run/attackbox"

# 挂载隔离的 tmpfs home 空间并初始化 parrot 用户
mount -t tmpfs tmpfs "%s/home" -o rw,nosuid,nodev,mode=0755
mkdir -p "%s/home/parrot"
if [ -d /home/parrot ]; then
  cp -a /home/parrot/. "%s/home/parrot/" 2>/dev/null || true
else
  cp -r /etc/skel/. "%s/home/parrot/" 2>/dev/null || true
fi
chown -R parrot:parrot "%s/home/parrot"
chmod 0755 "%s/home/parrot"

# chroot 并以容器工业标准 tini / dumb-init 启动常驻保活与僵尸收割
exec chroot "%s" /bin/bash -c "
  hostname attackbox-%s
  touch /run/attackbox/core.ready

  INIT_BIN=\"\"
  for b in /usr/bin/tini /bin/tini /usr/bin/dumb-init /bin/dumb-init; do
    if [ -x \"\$b\" ]; then
      INIT_BIN=\"\$b\"
      break
    fi
  done

  if [ -n \"\$INIT_BIN\" ]; then
    exec \"\$INIT_BIN\" -s -- sleep infinity
  else
    set -m
    trap 'while wait -n 2>/dev/null; do :; done' CHLD
    while true; do
      sleep 86400 &
      wait \$! 2>/dev/null || true
    done
  fi
"
`, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, b.RunDir, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, rootfs, b.ID)

	// 4. 通过 ip netns + unshare 组合指令拉起全新全命名空间
	cmd := exec.Command("ip", "netns", "exec", netns,
		"unshare", "--mount", "--uts", "--ipc", "--pid", "--fork", "--kill-child",
		"/bin/bash", "-c", initScript)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start unshare init: %w", err)
	}

	// 异步自动收割退出状态，彻底杜绝宿主机僵尸进程残留
	go func() {
		_ = cmd.Wait()
	}()

	unsharePID := cmd.Process.Pid
	_ = os.WriteFile(filepath.Join(b.RunDir, "unshare.pid"), []byte(strconv.Itoa(unsharePID)), 0644)

	// 5. 等待 core.ready 并解析沙箱内 PID 1 在宿主机的真实 PID
	coreReady := filepath.Join(b.RunDir, "core.ready")
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(coreReady); err == nil {
			if childPID, err := findChildPID(unsharePID); err == nil {
				_ = os.WriteFile(filepath.Join(b.RunDir, "init.pid"), []byte(strconv.Itoa(childPID)), 0644)
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return fmt.Errorf("sandbox core failed to become ready within timeout")
}

// Ensure 保证核心沙箱处于运行状态 (线程安全，支持并发调用与双重检测)
func (m *Manager) Ensure(ctx context.Context, b *config.Box) error {
	mu := m.boxLock(b.ID)
	mu.Lock()
	defer mu.Unlock()

	if m.Status(ctx, b).Running() {
		return nil
	}
	return m.startLocked(ctx, b)
}

// EnsureGUI 保证图形桌面服务处于运行状态 (线程安全，按需懒加载)
func (m *Manager) EnsureGUI(ctx context.Context, b *config.Box) error {
	mu := m.boxLock(b.ID)
	mu.Lock()
	defer mu.Unlock()

	if !m.Status(ctx, b).Running() {
		if err := m.startLocked(ctx, b); err != nil {
			return err
		}
	}

	guiSock := filepath.Join(b.RunDir, "gui.sock")
	if _, err := os.Stat(guiSock); err == nil {
		return nil
	}

	rootfs := m.Snapshots.PathFor(b.ID)
	return m.startGUI(ctx, b, rootfs)
}

// startGUI 统一通过 nsenter 穿透全命名空间拉起 TigerVNC 与 MATE 桌面
func (m *Manager) startGUI(ctx context.Context, b *config.Box, rootfs string) error {
	guiSock := filepath.Join(b.RunDir, "gui.sock")
	dispStr := fmt.Sprintf(":%d", b.Display)

	initPidFile := filepath.Join(b.RunDir, "init.pid")
	data, err := os.ReadFile(initPidFile)
	if err != nil {
		return fmt.Errorf("read init pid: %w", err)
	}
	initPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || initPID <= 0 {
		return fmt.Errorf("invalid init pid: %s", string(data))
	}

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

	// 统一通过 nsenter 将桌面启动脚本注入至沙箱环境中
	cmd := exec.Command("nsenter",
		"--target", strconv.Itoa(initPID),
		"--mount", "--uts", "--ipc", "--net", "--pid",
		"--root", "--wd",
		"/bin/bash", "-c", guiScript)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gui: %w", err)
	}

	// 异步自动收割 GUI 进程，杜绝僵尸进程
	go func() {
		_ = cmd.Wait()
	}()

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
	mu := m.boxLock(b.ID)
	mu.Lock()
	defer mu.Unlock()
	return m.stopGUILocked(ctx, b)
}

func (m *Manager) stopGUILocked(ctx context.Context, b *config.Box) error {
	guiPidFile := filepath.Join(b.RunDir, "gui.pid")
	if data, err := os.ReadFile(guiPidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-TERM", strconv.Itoa(pid)).Run()
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
		_ = os.Remove(guiPidFile)
	}

	// 仅在沙箱内部 PID Namespace 清理 TigerVNC 与 MATE 进程，绝不影响其他租户
	initPidFile := filepath.Join(b.RunDir, "init.pid")
	if data, err := os.ReadFile(initPidFile); err == nil {
		if initPID, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && initPID > 0 {
			_ = exec.CommandContext(ctx, "nsenter",
				"--target", strconv.Itoa(initPID),
				"--mount", "--uts", "--ipc", "--net", "--pid",
				"--root", "--wd",
				"pkill", "-f", "Xtigervnc").Run()
			_ = exec.CommandContext(ctx, "nsenter",
				"--target", strconv.Itoa(initPID),
				"--mount", "--uts", "--ipc", "--net", "--pid",
				"--root", "--wd",
				"pkill", "-f", "mate-session").Run()
		}
	}

	_ = os.Remove(filepath.Join(b.RunDir, "gui.sock"))
	rootfs := m.Snapshots.PathFor(b.ID)
	_ = os.Remove(filepath.Join(rootfs, fmt.Sprintf("tmp/.X11-unix/X%d", b.Display)))
	_ = os.Remove(filepath.Join(rootfs, fmt.Sprintf("tmp/.X%d-lock", b.Display)))
	return nil
}

// Kick 终止沙箱全部进程、注销网络并物理销毁快照 (内核级级联清理，线程安全)
func (m *Manager) Kick(ctx context.Context, b *config.Box) error {
	mu := m.boxLock(b.ID)
	mu.Lock()
	defer mu.Unlock()
	return m.kickLocked(ctx, b)
}

func (m *Manager) kickLocked(ctx context.Context, b *config.Box) error {
	// 1. 终止 GUI 进程
	_ = m.stopGUILocked(ctx, b)

	// 2. 核心处决：向沙箱 PID 1 (tini) 发送 SIGKILL
	// Linux 内核规则：PID 1 死亡时，内核自动向该 PID Namespace 内的一切子孙进程广播 SIGKILL
	initPidFile := filepath.Join(b.RunDir, "init.pid")
	if data, err := os.ReadFile(initPidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
	}

	// 处决外层 unshare 守护进程
	unsharePidFile := filepath.Join(b.RunDir, "unshare.pid")
	if data, err := os.ReadFile(unsharePidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			_ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).Run()
		}
	}

	// 等待进程完全消亡 (最多等待 1 秒)
	for i := 0; i < 10; i++ {
		if !m.Status(ctx, b).Running() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 3. 注销网络命名空间与网卡对
	netns := "ab-" + b.ID
	_ = exec.CommandContext(ctx, "ip", "netns", "del", netns).Run()
	_ = exec.CommandContext(ctx, "ip", "link", "del", "veth-"+b.ID+"-br").Run()

	// 4. 物理销毁快照
	// 说明：由于沙箱内的所有进程已被内核 SIGKILL 处死，其私有 Mount Namespace 随之自动销毁，
	// 没有任何文件或挂载占用，因此 btrfs subvolume delete 绝不会触发 target is busy 错误。
	_ = m.Snapshots.Destroy(b.ID)

	// 5. 清理运行时文件
	_ = os.Remove(filepath.Join(b.RunDir, "core.ready"))
	_ = os.Remove(filepath.Join(b.RunDir, "init.pid"))
	_ = os.Remove(filepath.Join(b.RunDir, "unshare.pid"))
	_ = os.Remove(filepath.Join(b.RunDir, "gui.sock"))
	_ = os.Remove(filepath.Join(b.RunDir, "gui.pid"))

	return nil
}
