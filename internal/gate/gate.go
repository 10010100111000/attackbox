// Package gate is the single-port web gateway: auth, routing and bridging.
package gate

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/internal/auth"
	"attackbox/internal/box"
	"attackbox/internal/cfg"
)

//go:embed templates/*.html
var tmplFS embed.FS

type session struct {
	User    string
	Role    string
	BoxID   string
	Expires time.Time
}

type BroadcastConfig struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

type Gate struct {
	cfg       *cfg.Config
	boxes     *box.Manager
	authStore *auth.Store
	tmpl      *template.Template

	mu        sync.Mutex
	sess      map[string]*session // cookieValue -> session
	userBoxes map[string]string   // username -> boxID
	broadcast BroadcastConfig

	cliMu     sync.Mutex
	activeCLI map[string]*websocket.Conn // boxID -> current active CLI connection

	guiMu     sync.Mutex
	activeGUI map[string]*websocket.Conn // boxID -> current active GUI connection

	novnc http.Handler
	xterm http.Handler
}

func New(c *cfg.Config, m *box.Manager) (*Gate, error) {
	t, err := template.ParseFS(tmplFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	dbPath := "/var/lib/attackbox/attackbox.db"
	store, err := auth.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("init auth store at %s: %w", dbPath, err)
	}

	return &Gate{
		cfg:       c,
		boxes:     m,
		authStore: store,
		tmpl:      t,
		sess:      map[string]*session{},
		userBoxes: map[string]string{},
		activeCLI: map[string]*websocket.Conn{},
		activeGUI: map[string]*websocket.Conn{},
		broadcast: BroadcastConfig{
			Enabled: true,
			Message: "欢迎使用 AttackBox 靶场工作台，实验结束后请随手关闭不再使用的环境。",
		},
		novnc: http.FileServer(http.Dir(c.NovncDir)),
		xterm: http.FileServer(http.Dir(c.XtermDir)),
	}, nil
}

func (g *Gate) Handler() http.Handler {
	mux := http.NewServeMux()

	// 页面渲染路由
	mux.HandleFunc("GET /{$}", g.indexPage)
	mux.HandleFunc("GET /login", g.indexPage)
	mux.HandleFunc("GET /logout", g.logout)

	// Web 认证 API
	mux.HandleFunc("POST /api/login", g.apiLogin)
	mux.HandleFunc("POST /api/register", g.apiRegister)
	mux.HandleFunc("POST /api/logout", g.apiLogout)
	mux.HandleFunc("GET /api/me", g.apiMe)

	// 渗透环境控制 API (支持 CLI、GUI 独立启停及一键销毁)
	mux.HandleFunc("POST /api/env/toggle", g.auth(g.apiCliToggle))
	mux.HandleFunc("POST /api/env/cli/toggle", g.auth(g.apiCliToggle))
	mux.HandleFunc("POST /api/env/gui/toggle", g.auth(g.apiGuiToggle))
	mux.HandleFunc("POST /api/env/destroy", g.auth(g.apiEnvDestroy))
	mux.HandleFunc("GET /api/env/status", g.auth(g.apiEnvStatus))

	// 管理员专属 API (需要 role == 'admin')
	mux.HandleFunc("GET /api/admin/metrics", g.adminAuth(g.apiAdminMetrics))
	mux.HandleFunc("GET /api/admin/users", g.adminAuth(g.apiAdminListUsers))
	mux.HandleFunc("POST /api/admin/users/create", g.adminAuth(g.apiAdminCreateUser))
	mux.HandleFunc("POST /api/admin/users/toggle", g.adminAuth(g.apiAdminToggleUser))
	mux.HandleFunc("POST /api/admin/users/reset-pass", g.adminAuth(g.apiAdminResetPass))
	mux.HandleFunc("POST /api/admin/users/delete", g.adminAuth(g.apiAdminDeleteUser))
	mux.HandleFunc("POST /api/admin/broadcast", g.adminAuth(g.apiAdminBroadcast))

	// 受保护的静态资源与 WebSocket 桥接
	mux.HandleFunc("GET /vnc", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/vnc/vnc.html?path=vnc/websockify", http.StatusSeeOther)
	})
	mux.Handle("GET /vnc/", http.StripPrefix("/vnc/", g.novnc))
	mux.HandleFunc("GET /vnc/websockify", g.auth(g.vncWS))
	mux.HandleFunc("GET /websockify", g.auth(g.vncWS))

	mux.Handle("GET /xterm/", http.StripPrefix("/xterm/", g.xterm))
	mux.HandleFunc("GET /term", g.auth(g.termPage))
	mux.HandleFunc("GET /term/ws", g.auth(g.termWS))

	return mux
}

// Run 定期清理过期 session
func (g *Gate) Run(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			g.mu.Lock()
			now := time.Now()
			for k, s := range g.sess {
				if now.After(s.Expires) {
					// 超时自动关闭环境
					if s.BoxID != "" {
						if b := g.cfg.BoxByID(s.BoxID); b != nil {
							log.Printf("session expired for %s, tearing down %s", s.User, b.ID)
							_ = g.boxes.Kick(ctx, b)
						}
						delete(g.userBoxes, s.User)
					}
					delete(g.sess, k)
				}
			}
			g.mu.Unlock()
		}
	}
}

// ---------- 鉴权拦截器 ----------

func (g *Gate) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := g.lookup(r)
		if s == nil {
			if strings.HasSuffix(r.URL.Path, "/websockify") || r.URL.Path == "/term/ws" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func (g *Gate) adminAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := g.lookup(r)
		if s == nil || s.Role != "admin" {
			http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (g *Gate) lookup(r *http.Request) *session {
	c, err := r.Cookie("ab")
	if err != nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.sess[c.Value]
	if s == nil {
		return nil
	}
	if time.Now().After(s.Expires) {
		delete(g.sess, c.Value)
		return nil
	}
	return s
}

func (g *Gate) boxFor(r *http.Request) (*cfg.Box, *session) {
	s := g.lookup(r)
	if s == nil {
		return nil, nil
	}
	g.mu.Lock()
	boxID := g.userBoxes[s.User]
	if boxID == "" {
		boxID = s.BoxID
	}
	if boxID == "" {
		// 方案 A 懒加载：如果用户尚未分配槽位，自动寻找空闲槽位绑定
		inUse := make(map[string]bool)
		for _, bid := range g.userBoxes {
			inUse[bid] = true
		}
		for i := range g.cfg.Boxes {
			if !inUse[g.cfg.Boxes[i].ID] {
				boxID = g.cfg.Boxes[i].ID
				g.userBoxes[s.User] = boxID
				s.BoxID = boxID
				break
			}
		}
	}
	g.mu.Unlock()

	if boxID == "" {
		boxID = "box1"
	}
	return g.cfg.BoxByID(boxID), s
}

// ---------- 页面渲染 ----------

func (g *Gate) indexPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := g.tmpl.ExecuteTemplate(w, "dash.html", map[string]any{}); err != nil {
		log.Printf("render dash.html: %v", err)
	}
}

func (g *Gate) termPage(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil {
		http.Error(w, "environment not running", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = g.tmpl.ExecuteTemplate(w, "term.html", map[string]any{"User": s.User, "Box": b})
}

// ---------- 认证 API ----------

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (g *Gate) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	u, err := g.authStore.Authenticate(req.Username, req.Password)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "用户名或密码错误"})
		return
	}

	tok := randHex(32)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    u.Username,
		Role:    u.Role,
		Expires: time.Now().Add(24 * time.Hour),
	}
	g.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, map[string]any{
		"success": true,
		"user": map[string]any{
			"username": u.Username,
			"role":     u.Role,
		},
	})
}

func (g *Gate) apiRegister(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}

	if len(req.Username) < 3 || len(req.Password) < 6 {
		writeJSON(w, map[string]any{"success": false, "error": "用户名至少3位，密码至少6位"})
		return
	}

	u, err := g.authStore.Register(req.Username, req.Password)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "该用户名已被注册"})
		return
	}

	tok := randHex(32)
	g.mu.Lock()
	g.sess[tok] = &session{
		User:    u.Username,
		Role:    u.Role,
		Expires: time.Now().Add(24 * time.Hour),
	}
	g.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, map[string]any{
		"success": true,
		"user": map[string]any{
			"username": u.Username,
			"role":     u.Role,
		},
	})
}

func (g *Gate) apiLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("ab")
	if err == nil {
		g.mu.Lock()
		delete(g.sess, c.Value)
		g.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "ab",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gate) logout(w http.ResponseWriter, r *http.Request) {
	g.apiLogout(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (g *Gate) apiMe(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	if s == nil {
		writeJSON(w, map[string]any{"loggedIn": false})
		return
	}

	b, _ := g.boxFor(r)
	cliRunning := false
	guiRunning := false
	boxID := ""
	if b != nil {
		cliRunning, guiRunning = g.boxes.DetailedStatus(r.Context(), b)
		boxID = b.ID
	}

	writeJSON(w, map[string]any{
		"loggedIn":   true,
		"user":       map[string]any{"username": s.User, "role": s.Role},
		"boxId":      boxID,
		"cliRunning": cliRunning,
		"guiRunning": guiRunning,
		"envRunning": cliRunning || guiRunning,
		"broadcast":  g.broadcast,
	})
}

// ---------- 渗透环境控制 (CLI/GUI 独立启停与一键释放) ----------

func (g *Gate) apiCliToggle(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil || s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	coreRunning, guiRunning := g.boxes.DetailedStatus(ctx, b)
	if coreRunning {
		// 如果 GUI 未运行，关闭 CLI 即彻底释放沙箱；若 GUI 在运行，仅保留 GUI
		if !guiRunning {
			_ = g.boxes.Kick(ctx, b)
			g.mu.Lock()
			delete(g.userBoxes, s.User)
			s.BoxID = ""
			g.mu.Unlock()
			writeJSON(w, map[string]any{"success": true, "cliRunning": false, "guiRunning": false})
			return
		}
		writeJSON(w, map[string]any{"success": true, "cliRunning": false, "guiRunning": guiRunning})
		return
	}

	// 启动 CLI (核心沙箱，毫秒级)
	if err := g.boxes.Ensure(ctx, b); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "启动 CLI 沙箱失败: " + err.Error()})
		return
	}

	g.mu.Lock()
	g.userBoxes[s.User] = b.ID
	s.BoxID = b.ID
	g.mu.Unlock()

	writeJSON(w, map[string]any{"success": true, "cliRunning": true, "guiRunning": guiRunning})
}

func (g *Gate) apiGuiToggle(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil || s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	coreRunning, guiRunning := g.boxes.DetailedStatus(ctx, b)
	if guiRunning {
		// 独立停止 GUI 桌面并切断 GUI 连接
		g.guiMu.Lock()
		if conn, ok := g.activeGUI[b.ID]; ok && conn != nil {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "gui stopped"),
				time.Now().Add(time.Second),
			)
			_ = conn.Close()
			delete(g.activeGUI, b.ID)
		}
		g.guiMu.Unlock()

		_ = g.boxes.StopGUI(ctx, b)
		writeJSON(w, map[string]any{"success": true, "cliRunning": coreRunning, "guiRunning": false})
		return
	}

	// 启动 GUI 桌面 (按需启动核心与桌面)
	if err := g.boxes.EnsureGUI(ctx, b); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "启动 GUI 桌面失败: " + err.Error()})
		return
	}

	g.mu.Lock()
	g.userBoxes[s.User] = b.ID
	s.BoxID = b.ID
	g.mu.Unlock()

	writeJSON(w, map[string]any{"success": true, "cliRunning": true, "guiRunning": true})
}

// closeBoxSessions 彻底切断指定沙箱的所有活跃 WebSocket 会话 (CLI 与 GUI)
func (g *Gate) closeBoxSessions(boxID string) {
	g.cliMu.Lock()
	for key, conn := range g.activeCLI {
		if strings.HasPrefix(key, boxID+":") || key == boxID {
			if conn != nil {
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "environment destroyed"),
					time.Now().Add(time.Second),
				)
				_ = conn.Close()
			}
			delete(g.activeCLI, key)
		}
	}
	g.cliMu.Unlock()

	g.guiMu.Lock()
	if conn, ok := g.activeGUI[boxID]; ok && conn != nil {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "environment destroyed"),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
		delete(g.activeGUI, boxID)
	}
	g.guiMu.Unlock()
}

func (g *Gate) apiEnvDestroy(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil || s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. 彻底断开所有活跃终端与桌面 WebSocket 连接，触发底层 PTY 进程与套接字立即销毁
	g.closeBoxSessions(b.ID)

	// 2. 终止沙箱进程、命名空间、卸载挂载并删除快照
	_ = g.boxes.Kick(ctx, b)
	g.mu.Lock()
	delete(g.userBoxes, s.User)
	s.BoxID = ""
	g.mu.Unlock()

	writeJSON(w, map[string]any{"success": true, "cliRunning": false, "guiRunning": false, "envRunning": false})
}


func (g *Gate) apiEnvStatus(w http.ResponseWriter, r *http.Request) {
	s := g.lookup(r)
	if s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	g.mu.Lock()
	boxID := g.userBoxes[s.User]
	g.mu.Unlock()

	running := false
	if boxID != "" {
		if b := g.cfg.BoxByID(boxID); b != nil {
			running = g.boxes.Status(r.Context(), b).Running()
		}
	}

	writeJSON(w, map[string]any{"running": running, "box": boxID})
}

// ---------- 管理员专属 API ----------

func (g *Gate) apiAdminMetrics(w http.ResponseWriter, r *http.Request) {
	cpuPercent := getCPUPercent()
	memUsed, memTotal := getMemUsageGB()
	diskUsed, diskTotal := getDiskUsageGB("/var/lib/attackbox")

	runningCount := 0
	for i := range g.cfg.Boxes {
		if g.boxes.Status(r.Context(), &g.cfg.Boxes[i]).Running() {
			runningCount++
		}
	}

	writeJSON(w, map[string]any{
		"vpsOnline":    true,
		"cpuPercent":   cpuPercent,
		"memUsedGB":    memUsed,
		"memTotalGB":   memTotal,
		"diskUsedGB":   diskUsed,
		"diskTotalGB":  diskTotal,
		"runningBoxes": runningCount,
	})
}

func (g *Gate) apiAdminListUsers(w http.ResponseWriter, r *http.Request) {
	list, err := g.authStore.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (g *Gate) apiAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}
	_, err := g.authStore.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gate) apiAdminToggleUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	_, err := g.authStore.ToggleActive(req.Username)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gate) apiAdminResetPass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	err := g.authStore.UpdatePassword(req.Username, req.Password)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gate) apiAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效的请求格式"})
		return
	}
	err := g.authStore.DeleteUser(req.Username)
	if err != nil {
		writeJSON(w, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (g *Gate) apiAdminBroadcast(w http.ResponseWriter, r *http.Request) {
	var req BroadcastConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"success": false, "error": "无效请求"})
		return
	}
	g.mu.Lock()
	g.broadcast = req
	g.mu.Unlock()
	writeJSON(w, map[string]any{"success": true})
}

// ---------- 内部工具函数 ----------

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}

func randHex(n int) string {
	b := make([]byte, n/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getCPUPercent() int {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 15
	}
	parts := strings.Fields(string(data))
	if len(parts) > 0 {
		load, _ := strconv.ParseFloat(parts[0], 64)
		pct := int(load * 25)
		if pct > 100 {
			pct = 100
		}
		if pct < 5 {
			pct = 5
		}
		return pct
	}
	return 15
}

func getMemUsageGB() (float64, float64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 3.2, 8.0
	}
	defer file.Close()

	var totalKB, availKB float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				totalKB, _ = strconv.ParseFloat(fields[1], 64)
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				availKB, _ = strconv.ParseFloat(fields[1], 64)
			}
		}
	}
	if totalKB == 0 {
		return 3.2, 8.0
	}
	usedKB := totalKB - availKB
	totalGB := math.Round(totalKB/1024/1024*10) / 10
	usedGB := math.Round(usedKB/1024/1024*10) / 10
	return usedGB, totalGB
}

func getDiskUsageGB(path string) (float64, float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 45.0, 100.0
	}
	total := float64(stat.Blocks) * float64(stat.Bsize)
	free := float64(stat.Bfree) * float64(stat.Bsize)
	used := total - free
	totalGB := math.Round(total/(1024*1024*1024)*10) / 10
	usedGB := math.Round(used/(1024*1024*1024)*10) / 10
	return usedGB, totalGB
}
