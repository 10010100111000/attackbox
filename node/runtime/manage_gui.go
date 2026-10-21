package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func checkGUISocket(sockPath string) bool {
	conn, err := net.DialTimeout("unix", sockPath, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (m *RuntimeManager) GetGUISocketPath(sandboxID string) (string, error) {
	snapshot, ok := m.GetSnapshot(sandboxID)
	if !ok || snapshot.PID <= 0 || snapshot.State != "running" {
		return "", fmt.Errorf("sandbox %s is not active", sandboxID)
	}
	return filepath.Join(fmt.Sprintf("/proc/%d/root", snapshot.PID), "run/attackbox/gui.sock"), nil
}

// The desktop supervisor stays inside the sandbox and exits if either VNC or
// MATE exits. Closing a viewer does not stop this process. Full sandbox teardown
// uses the existing PID 1/cgroup cleanup, just like every other sandbox process.
const guiScript = `set -e
install -d -m 1777 /tmp/.ICE-unix /tmp/.X11-unix
install -d -m 0700 -o box -g box /run/attackbox /tmp/runtime-box
rm -f /run/attackbox/gui.sock /tmp/.X11-unix/X1 /tmp/.X1-lock
if [ ! -f /home/box/.config/dconf/user ] || [ $(wc -c < /home/box/.config/dconf/user 2>/dev/null || echo 0) -lt 10000 ]; then
  cp -r /etc/skel/. /home/box/ 2>/dev/null || true
  chown -R box:box /home/box 2>/dev/null || true
fi
exec runuser -u box -- /bin/bash <<'DESKTOP'
set -e
export DISPLAY=:1
export XDG_RUNTIME_DIR=/tmp/runtime-box
export XDG_CURRENT_DESKTOP=MATE
export XDG_SESSION_TYPE=x11
export HOME=/home/box
cd "$HOME"
vnc=
desktop=
trap 'kill $vnc $desktop 2>/dev/null || true; rm -f /run/attackbox/gui.sock' EXIT
trap 'exit 0' TERM INT
/usr/bin/Xtigervnc :1 -rfbunixpath /run/attackbox/gui.sock -rfbunixmode 0600 \
 -rfbport -1 -nolisten tcp -SecurityTypes None -AlwaysShared -UseBlacklist=0 \
 -FrameRate 30 -CompareFB 1 \
 -geometry 1280x800 -depth 24 &
vnc=$!
for i in $(seq 1 80); do
 kill -0 "$vnc" || exit 1
 [ -S /tmp/.X11-unix/X1 ] && [ -S /run/attackbox/gui.sock ] && break
 sleep 0.1
done
[ -S /tmp/.X11-unix/X1 ] && [ -S /run/attackbox/gui.sock ] || exit 1
dbus-run-session /bin/bash -c 'gsettings set org.mate.Marco.general compositing-manager false 2>/dev/null || true; gsettings set org.mate.interface enable-animations false 2>/dev/null || true; exec /usr/bin/mate-session' &
desktop=$!
wait -n "$vnc" "$desktop"
DESKTOP
`

// EnsureGUI shares the instance operation lock with creation and destruction;
// one user's startup never holds a global lock needed by another user.
func (m *RuntimeManager) EnsureGUI(ctx context.Context, sandboxID string) error {
	m.mu.RLock()
	rec := m.sandboxes[sandboxID]
	m.mu.RUnlock()
	if rec == nil {
		return ErrSandboxNotFound
	}
	rec.opMu.Lock()
	defer rec.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.RLock()
	valid := m.sandboxes[sandboxID] == rec && rec.State == "running" && rec.PID > 0
	targetPID := rec.PID
	m.mu.RUnlock()
	if !valid {
		return fmt.Errorf("sandbox %s is not running", sandboxID)
	}
	sock := filepath.Join(fmt.Sprintf("/proc/%d/root", targetPID), "run/attackbox/gui.sock")
	if checkGUISocket(sock) {
		m.mu.Lock()
		if !rec.GUIRunning {
			rec.GUIRunning = true
			m.revision++
			rec.Revision = m.revision
		}
		m.mu.Unlock()
		return nil
	}
	// An owned supervisor may still be shutting down. Never launch two desktops
	// into the same display just because its socket is temporarily unavailable.
	m.mu.RLock()
	launching := rec.guiCmd != nil
	m.mu.RUnlock()
	if launching {
		return fmt.Errorf("desktop is restarting; reconnect shortly")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cg, err := os.Open(m.CgroupPath(sandboxID))
	if err != nil {
		return fmt.Errorf("open desktop cgroup: %w", err)
	}
	defer cg.Close()
	cmd := exec.Command(exe, "__enter_root__", strconv.Itoa(targetPID), "/bin/bash", "-c", guiScript)
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "LANG=en_US.UTF-8", "HOME=/root"}
	cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: int(cg.Fd()), Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start desktop: %w", err)
	}
	m.mu.Lock()
	rec.guiCmd = cmd
	m.mu.Unlock()
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if rec.guiCmd == cmd {
			rec.guiCmd = nil
			if rec.GUIRunning {
				rec.GUIRunning = false
				m.revision++
				rec.Revision = m.revision
			}
		}
		close(done)
		m.mu.Unlock()
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-done:
			return fmt.Errorf("desktop exited before becoming ready")
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			return ctx.Err()
		case <-timeout.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			return fmt.Errorf("desktop startup timed out")
		case <-ticker.C:
			if !checkGUISocket(sock) {
				continue
			}
			m.mu.Lock()
			select {
			case <-done:
				m.mu.Unlock()
				return fmt.Errorf("desktop exited before becoming ready")
			default:
			}
			rec.GUIRunning = true
			m.revision++
			rec.Revision = m.revision
			m.mu.Unlock()
			return nil
		}
	}
}
