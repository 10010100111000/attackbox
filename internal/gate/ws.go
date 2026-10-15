package gate

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// Cloudflare drops idle WebSockets after roughly 100 seconds, and RFB only
// speaks when the screen changes. 25s pings keep the connection alive.
const pingEvery = 25 * time.Second

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1 << 15,
	WriteBufferSize: 1 << 15,
	Subprotocols:    []string{"binary"},
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func keepalive(c *websocket.Conn, stop <-chan struct{}) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			_ = c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
		}
	}
}

// vncWS bridges a browser WebSocket to the box RFB unix socket, byte for byte.
// 方案 A 懒加载：用户访问 GUI 时，按需拉起 TigerVNC 与 MATE 桌面
func (g *Gate) vncWS(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil || s == nil {
		http.Error(w, "no box", http.StatusInternalServerError)
		return
	}

	// 按需确保 GUI 桌面已经启动
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := g.boxes.EnsureGUI(ctx, b); err != nil {
		log.Printf("vncWS ensure GUI %s: %v", b.ID, err)
		http.Error(w, "cannot start desktop: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("vncWS upgrade error: %v", err)
		return
	}
	defer c.Close()

	// 严格单活跃会话：若已有活动 GUI 窗口，向其发送 4001 Preempted 并关闭
	g.guiMu.Lock()
	if oldConn, exists := g.activeGUI[b.ID]; exists && oldConn != nil {
		_ = oldConn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "preempted"),
			time.Now().Add(time.Second),
		)
		_ = oldConn.Close()
	}
	g.activeGUI[b.ID] = c
	g.guiMu.Unlock()

	defer func() {
		g.guiMu.Lock()
		if g.activeGUI[b.ID] == c {
			delete(g.activeGUI, b.ID)
		}
		g.guiMu.Unlock()
	}()

	conn, err := net.DialTimeout("unix", b.GuiSocket, 5*time.Second)
	if err != nil {
		log.Printf("vnc: dial %s: %v", b.GuiSocket, err)
		_ = c.WriteMessage(websocket.TextMessage, []byte("box display is not running"))
		return
	}
	defer conn.Close()

	stop := make(chan struct{})
	go keepalive(c, stop)
	defer close(stop)

	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.BinaryMessage && mt != websocket.TextMessage {
				continue
			}
			if _, err := conn.Write(msg); err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 1<<15)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if werr := c.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	<-done
}

// termWS bridges terminal WebSocket to a real PTY inside the sandbox.
// 方案 A 懒加载：用户访问 CLI 时，毫秒级快速拉起沙箱 Core 并建立双向交互
func (g *Gate) termWS(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil || s == nil {
		http.Error(w, "no box", http.StatusInternalServerError)
		return
	}

	// 按需确保核心沙箱就绪（毫秒级）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := g.boxes.Ensure(ctx, b); err != nil {
		log.Printf("termWS ensure core %s: %v", b.ID, err)
		http.Error(w, "cannot start sandbox: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	winID := r.URL.Query().Get("win")
	if winID == "" {
		winID = "default"
	}
	sessKey := b.ID + ":" + winID

	// 窗口会话管理：若该特定窗口已有连接，发送 4001 并关闭旧连接
	g.cliMu.Lock()
	if oldConn, exists := g.activeCLI[sessKey]; exists && oldConn != nil {
		_ = oldConn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "preempted"),
			time.Now().Add(time.Second),
		)
		_ = oldConn.Close()
	}
	g.activeCLI[sessKey] = c
	g.cliMu.Unlock()

	defer func() {
		g.cliMu.Lock()
		if g.activeCLI[sessKey] == c {
			delete(g.activeCLI, sessKey)
		}
		g.cliMu.Unlock()
	}()

	stop := make(chan struct{})
	go keepalive(c, stop)
	defer close(stop)

	rootfs := g.boxes.Snapshots.PathFor(b.ID)
	netns := "ab-" + b.ID

	// 原生轻量解耦 PTY：直接拉起隔离命名空间与 chroot 下的标准登录 Shell，彻底废除 Tmux 依赖与状态栏杂质
	cmd := exec.Command("/usr/bin/ip", "netns", "exec", netns,
		"/usr/sbin/chroot", rootfs,
		"/usr/bin/su", "-", "parrot")

	cmd.Env = []string{
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 35, Cols: 110})
	if err != nil {
		log.Printf("term: pty.Start: %v", err)
		_ = c.WriteMessage(websocket.TextMessage, []byte("cannot start terminal: "+err.Error()))
		return
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = f.Close()
	}()

	// PTY 数据流 -> WebSocket 输出
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				if werr := c.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				_ = c.Close()
				return
			}
		}
	}()

	// WebSocket 键盘/缩放输入 -> PTY
	for {
		mt, msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		if mt == websocket.TextMessage {
			var sz struct {
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(msg, &sz) == nil && sz.Cols > 0 && sz.Rows > 0 {
				_ = pty.Setsize(f, &pty.Winsize{Cols: sz.Cols, Rows: sz.Rows})
				continue
			}
		}
		if _, err := f.Write(msg); err != nil {
			return
		}
	}
}
