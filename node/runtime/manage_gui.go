package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// checkGUISocket 探测 Unix Socket 是否真正接受连接，防止 Xtigervnc 崩溃后的陈旧套接字伪存活
func checkGUISocket(sockPath string, cleanIfDead bool) bool {
	conn, err := net.DialTimeout("unix", sockPath, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	if cleanIfDead {
		_ = os.Remove(sockPath)
	}
	return false
}

// GetGUISocketPath 获取沙箱内部 Unix Socket 在宿主机上的实际可访问路径
func (m *RuntimeManager) GetGUISocketPath(sandboxID string) (string, error) {
	m.mu.RLock()
	rec, ok := m.sandboxes[sandboxID]
	m.mu.RUnlock()
	if !ok || rec.PID <= 0 {
		return "", fmt.Errorf("sandbox %s is not active", sandboxID)
	}
	return filepath.Join(fmt.Sprintf("/proc/%d/root", rec.PID), "run/attackbox/gui.sock"), nil
}

// IsGUIRunning 查询沙箱内部的图形远程桌面是否处于真实运行状态
func (m *RuntimeManager) IsGUIRunning(sandboxID string) bool {
	sockPath, err := m.GetGUISocketPath(sandboxID)
	if err != nil {
		return false
	}
	return checkGUISocket(sockPath, false)
}

// EnsureGUI 保证指定沙箱内部的 TigerVNC 与 MATE 桌面服务处于运行就绪状态 (并发安全懒加载)
func (m *RuntimeManager) EnsureGUI(ctx context.Context, sandboxID string) error {
	m.guiMu.Lock()
	defer m.guiMu.Unlock()

	m.mu.RLock()
	rec, ok := m.sandboxes[sandboxID]
	m.mu.RUnlock()
	if !ok || rec.PID <= 0 || rec.State != "running" {
		return fmt.Errorf("sandbox %s is not in running state", sandboxID)
	}

	targetPID := rec.PID
	procRoot := fmt.Sprintf("/proc/%d/root", targetPID)
	guiSockHostPath := filepath.Join(procRoot, "run/attackbox/gui.sock")

	// 1. 若套接字已存活且可连接，直接标记就绪
	if checkGUISocket(guiSockHostPath, true) {
		m.mu.Lock()
		rec.GUIRunning = true
		m.mu.Unlock()
		return nil
	}

	// 2. 清理沙箱内部可能残留的失效 X11 锁文件与旧 socket
	_ = os.Remove(guiSockHostPath)
	_ = os.Remove(filepath.Join(procRoot, "tmp/.X11-unix/X1"))
	_ = os.Remove(filepath.Join(procRoot, "tmp/.X1-lock"))

	// 3. 构建穿透沙箱命名空间的桌面拉起脚本
	guiScript := `
BOX_USER="box"
DISP=":1"
DISP_NUM=1

mkdir -p /tmp/.ICE-unix /tmp/.X11-unix "/tmp/runtime-$BOX_USER" /run/attackbox
chmod 1777 /tmp/.ICE-unix /tmp/.X11-unix /run/attackbox
chmod 0700 "/tmp/runtime-$BOX_USER"
chown "$BOX_USER:$BOX_USER" "/tmp/runtime-$BOX_USER" 2>/dev/null || true
chown -R "$BOX_USER:" "/home/$BOX_USER" 2>/dev/null || true
chmod 0755 "/home/$BOX_USER" 2>/dev/null || true
rm -f "/home/$BOX_USER/.ICEauthority" "/home/$BOX_USER/.Xauthority" /tmp/.ICE-unix/* 2>/dev/null || true

/usr/bin/Xtigervnc "$DISP" -rfbunixpath /run/attackbox/gui.sock -rfbunixmode 0666 \
  -SecurityTypes None -AlwaysShared -UseBlacklist=0 -geometry 1280x800 -depth 24 &

for i in $(seq 1 40); do
  [ -S "/tmp/.X11-unix/X$DISP_NUM" ] && [ -S /run/attackbox/gui.sock ] && break
  sleep 0.1
done

su - "$BOX_USER" -c "
  export DISPLAY=\"$DISP\"
  export XDG_RUNTIME_DIR=\"/tmp/runtime-$BOX_USER\"
  export XDG_CURRENT_DESKTOP=MATE
  export XDG_SESSION_TYPE=x11
  gsettings set org.mate.Marco.general compositing-manager false 2>/dev/null || true
  gsettings set org.mate.interface enable-animations false 2>/dev/null || true
  exec dbus-run-session /usr/bin/mate-session
"
`

	// 4. 统一通过 nsenter 穿透全命名空间注入并拉起 TigerVNC 与 MATE
	cmd := exec.Command("nsenter",
		"--target", strconv.Itoa(targetPID),
		"--mount", "--uts", "--ipc", "--net", "--pid",
		"--root", "--wd",
		"/bin/bash", "-c", guiScript)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gui in sandbox %s: %w", sandboxID, err)
	}

	// 异步自动收割 GUI 包装进程，杜绝孤儿/僵尸进程
	go func() {
		_ = cmd.Wait()
	}()

	// 5. 将桌面进程 PID 绑定至沙箱的 Cgroup v2 统一配额控制下
	if cmd.Process != nil {
		cgroupProcsPath := filepath.Join(m.CgroupPath(sandboxID), "cgroup.procs")
		_ = os.WriteFile(cgroupProcsPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644)
	}

	// 6. 轮询等待 gui.sock 真正创建就绪且可连接
	for i := 0; i < 40; i++ {
		if checkGUISocket(guiSockHostPath, false) {
			m.mu.Lock()
			rec.GUIRunning = true
			m.mu.Unlock()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return fmt.Errorf("gui socket %s not ready after launch", guiSockHostPath)
}

// StopGUI 仅终止沙箱内部的 TigerVNC 与 MATE 桌面进程，保持沙箱核心 CLI 继续存活
func (m *RuntimeManager) StopGUI(ctx context.Context, sandboxID string) error {
	m.guiMu.Lock()
	defer m.guiMu.Unlock()

	m.mu.RLock()
	rec, ok := m.sandboxes[sandboxID]
	m.mu.RUnlock()
	if !ok || rec.PID <= 0 {
		return nil
	}

	targetPID := rec.PID
	procRoot := fmt.Sprintf("/proc/%d/root", targetPID)
	guiSockHostPath := filepath.Join(procRoot, "run/attackbox/gui.sock")

	// 在沙箱 PID Namespace 内部清理 TigerVNC 与 MATE 进程，绝不影响其他租户或宿主机
	_ = exec.Command("nsenter",
		"--target", strconv.Itoa(targetPID),
		"--pid",
		"pkill", "-f", "Xtigervnc").Run()

	_ = exec.Command("nsenter",
		"--target", strconv.Itoa(targetPID),
		"--pid",
		"pkill", "-f", "mate-session").Run()

	_ = os.Remove(guiSockHostPath)

	m.mu.Lock()
	rec.GUIRunning = false
	m.mu.Unlock()

	return nil
}
