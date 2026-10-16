// Package gateway provides the single-port HTTP/WebSocket gateway: authentication, routing, and terminal/desktop bridging.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/internal/auth"
	"attackbox/internal/box"
	"attackbox/internal/config"
)

var ErrNoAvailableBox = errors.New("no available sandbox slot")

type session struct {
	User       string
	Role       string
	BoxID      string
	Expires    time.Time // 登录凭据 Cookie 有效期 (如 24h)
	BoxExpires time.Time // 沙箱环境运行寿命 (根据 Box.TTLMinutes 计算)
}

type BroadcastConfig struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

// safeConn 线程安全包装器，彻底防止并发调用 WriteMessage/WriteControl 触发 Gorilla WebSocket Panic
type safeConn struct {
	*websocket.Conn
	mu sync.Mutex
}

func newSafeConn(c *websocket.Conn) *safeConn {
	return &safeConn{Conn: c}
}

func (s *safeConn) WriteMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Conn.WriteMessage(messageType, data)
}

func (s *safeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Conn.WriteControl(messageType, data, deadline)
}

type Gateway struct {
	cfg       *config.Config
	boxes     *box.Manager
	authStore *auth.Store

	mu        sync.Mutex
	sess      map[string]*session // cookieValue -> session
	userBoxes map[string]string   // username -> boxID
	broadcast BroadcastConfig

	cliMu     sync.Mutex
	activeCLI map[string]*safeConn // sessKey -> current active CLI connection

	guiMu     sync.Mutex
	activeGUI map[string]*safeConn // boxID -> current active GUI connection

	novnc http.Handler
	xterm http.Handler
}

func New(c *config.Config, m *box.Manager) (*Gateway, error) {
	dbPath := "/var/lib/attackbox/attackbox.db"
	store, err := auth.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("init auth store at %s: %w", dbPath, err)
	}

	return &Gateway{
		cfg:       c,
		boxes:     m,
		authStore: store,
		sess:      map[string]*session{},
		userBoxes: map[string]string{},
		activeCLI: map[string]*safeConn{},
		activeGUI: map[string]*safeConn{},
		broadcast: BroadcastConfig{
			Enabled: true,
			Message: "欢迎使用 AttackBox 靶场工作台，实验结束后请随手关闭不再使用的环境。",
		},
		novnc: http.FileServer(http.Dir(c.NovncDir)),
		xterm: http.FileServer(http.Dir(c.XtermDir)),
	}, nil
}

// Run 定时扫描会话与沙箱 TTL，采用两阶段锁设计：锁内仅做微秒级内存摘除，锁外执行慢速 Kick I/O，杜绝阻塞全站
func (g *Gateway) Run(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			var toKick []*config.Box
			var toCloseBoxIDs []string

			// 第一阶段：原子快速内存状态更新 (微秒级立即释放锁)
			g.mu.Lock()
			for k, s := range g.sess {
				sessionExpired := now.After(s.Expires)
				boxTTLExpired := !s.BoxExpires.IsZero() && now.After(s.BoxExpires)

				if sessionExpired || boxTTLExpired {
					if s.BoxID != "" {
						if b := g.cfg.BoxByID(s.BoxID); b != nil {
							toKick = append(toKick, b)
							toCloseBoxIDs = append(toCloseBoxIDs, s.BoxID)
						}
						delete(g.userBoxes, s.User)
						s.BoxID = ""
						s.BoxExpires = time.Time{}
					}
					if sessionExpired {
						delete(g.sess, k)
					}
				}
			}
			g.mu.Unlock() // 立即释放全局锁，保证前台所有用户 HTTP API 绝对不卡顿

			// 第二阶段：在锁外执行慢速 I/O 与连接切断
			for _, boxID := range toCloseBoxIDs {
				g.closeBoxSessions(boxID)
			}
			for _, b := range toKick {
				_ = g.boxes.Kick(ctx, b)
			}
		}
	}
}

func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()

	// 页面路由
	mux.HandleFunc("GET /{$}", g.indexPage)
	mux.HandleFunc("GET /login", g.indexPage)
	mux.HandleFunc("GET /logout", g.logout)
	mux.HandleFunc("GET /term", g.auth(g.termPage))
	mux.HandleFunc("GET /gui", g.auth(g.guiPage))

	// REST API: 认证
	mux.HandleFunc("POST /api/login", g.apiLogin)
	mux.HandleFunc("POST /api/register", g.apiRegister)
	mux.HandleFunc("POST /api/logout", g.apiLogout)
	mux.HandleFunc("GET /api/me", g.apiMe)

	// REST API: 环境控制
	mux.HandleFunc("POST /api/env/toggle", g.auth(g.apiCliToggle))
	mux.HandleFunc("POST /api/env/cli/toggle", g.auth(g.apiCliToggle))
	mux.HandleFunc("POST /api/env/gui/toggle", g.auth(g.apiGuiToggle))
	mux.HandleFunc("POST /api/env/destroy", g.auth(g.apiEnvDestroy))
	mux.HandleFunc("GET /api/env/status", g.auth(g.apiEnvStatus))

	// REST API: 管理员
	mux.HandleFunc("GET /api/admin/metrics", g.adminAuth(g.apiAdminMetrics))
	mux.HandleFunc("GET /api/admin/users", g.adminAuth(g.apiAdminListUsers))
	mux.HandleFunc("POST /api/admin/users/create", g.adminAuth(g.apiAdminCreateUser))
	mux.HandleFunc("POST /api/admin/users/toggle", g.adminAuth(g.apiAdminToggleUser))
	mux.HandleFunc("POST /api/admin/users/reset-pass", g.adminAuth(g.apiAdminResetPass))
	mux.HandleFunc("POST /api/admin/users/delete", g.adminAuth(g.apiAdminDeleteUser))
	mux.HandleFunc("POST /api/admin/broadcast", g.adminAuth(g.apiAdminBroadcast))

	// 静态资源与核心库 (直接映射磁盘目录，改动后 F5 立即生效)
	staticDir := filepath.Join(g.cfg.WebDir, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	mux.Handle("GET /xterm/", http.StripPrefix("/xterm/", g.xterm))
	mux.Handle("GET /vnc/", http.StripPrefix("/vnc/", g.novnc))

	// WebSocket 桥接
	mux.HandleFunc("GET /term/ws", g.auth(g.termWS))
	mux.HandleFunc("GET /vnc/ws", g.auth(g.vncWS))

	return mux
}

// ---------- 鉴权中间件 ----------

func (g *Gateway) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := g.lookup(r)
		if s == nil {
			if strings.HasSuffix(r.URL.Path, "/ws") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func (g *Gateway) adminAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := g.lookup(r)
		if s == nil || s.Role != "admin" {
			http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (g *Gateway) lookup(r *http.Request) *session {
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

// currentBoxFor 纯只读查询，绝不擅自分配槽位 (供 apiMe 等状态检测使用，彻底解决 Bug B)
func (g *Gateway) currentBoxFor(r *http.Request) (*config.Box, *session) {
	s := g.lookup(r)
	if s == nil {
		return nil, nil
	}
	g.mu.Lock()
	boxID := g.userBoxes[s.User]
	if boxID == "" {
		boxID = s.BoxID
	}
	g.mu.Unlock()

	if boxID == "" {
		return nil, s
	}
	return g.cfg.BoxByID(boxID), s
}

// acquireBoxFor 按需独占分配沙箱槽位 (仅在明确启动环境时调用，彻底杜绝 Bug A 夺舍与 Bug B 虚占)
func (g *Gateway) acquireBoxFor(r *http.Request) (*config.Box, *session, error) {
	s := g.lookup(r)
	if s == nil {
		return nil, nil, errors.New("unauthorized")
	}

	g.mu.Lock()
	boxID := g.userBoxes[s.User]
	if boxID == "" {
		boxID = s.BoxID
	}

	if boxID != "" {
		g.mu.Unlock()
		b := g.cfg.BoxByID(boxID)
		if b != nil {
			return b, s, nil
		}
		g.mu.Lock()
	}

	// 查找当前真正空闲（未被用户绑定）的槽位
	inUse := make(map[string]bool)
	for _, bid := range g.userBoxes {
		inUse[bid] = true
	}

	for i := range g.cfg.Boxes {
		bid := g.cfg.Boxes[i].ID
		if !inUse[bid] {
			boxID = bid
			g.userBoxes[s.User] = boxID
			s.BoxID = boxID
			s.BoxExpires = time.Now().Add(time.Duration(g.cfg.Boxes[i].TTLMinutes) * time.Minute)
			g.mu.Unlock()
			return &g.cfg.Boxes[i], s, nil
		}
	}
	g.mu.Unlock()

	// 槽位已满，绝不回退至 box1！返回错误由调用方优雅向前端报错
	return nil, s, ErrNoAvailableBox
}

// evictUser 彻底注销指定用户的会话、掐断连接并立即销毁其沙箱
func (g *Gateway) evictUser(ctx context.Context, username string) {
	var boxIDToKick string

	g.mu.Lock()
	for tok, s := range g.sess {
		if s.User == username {
			delete(g.sess, tok)
		}
	}
	if boxID, exists := g.userBoxes[username]; exists {
		boxIDToKick = boxID
		delete(g.userBoxes, username)
	}
	g.mu.Unlock()

	if boxIDToKick != "" {
		g.closeBoxSessions(boxIDToKick)
		if b := g.cfg.BoxByID(boxIDToKick); b != nil {
			_ = g.boxes.Kick(ctx, b)
		}
	}
}

// ---------- 页面渲染 (从磁盘动态加载模板，修改 HTML 零编译立即生效) ----------

func (g *Gateway) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")

	tmplPattern := filepath.Join(g.cfg.WebDir, "templates", "*.html")
	t, err := template.ParseGlob(tmplPattern)
	if err != nil {
		log.Printf("parse templates from %s: %v", tmplPattern, err)
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render template %s: %v", name, err)
	}
}

func (g *Gateway) indexPage(w http.ResponseWriter, r *http.Request) {
	g.renderTemplate(w, "dash.html", map[string]any{})
}

func (g *Gateway) termPage(w http.ResponseWriter, r *http.Request) {
	b, s, err := g.acquireBoxFor(r)
	if err != nil || b == nil {
		http.Error(w, "靶场沙箱槽位已满，暂无空闲资源，请稍后再试或联系管理员", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	_ = g.boxes.Ensure(ctx, b)

	g.renderTemplate(w, "term.html", map[string]any{"User": s.User, "Box": b})
}

func (g *Gateway) guiPage(w http.ResponseWriter, r *http.Request) {
	b, s, err := g.acquireBoxFor(r)
	if err != nil || b == nil {
		http.Error(w, "靶场沙箱槽位已满，暂无空闲资源，请稍后再试或联系管理员", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	_ = g.boxes.EnsureGUI(ctx, b)

	g.renderTemplate(w, "gui.html", map[string]any{"User": s.User, "Box": b})
}
