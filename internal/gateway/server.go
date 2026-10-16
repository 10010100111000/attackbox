// Package gateway provides the single-port HTTP/WebSocket gateway: authentication, routing, and terminal/desktop bridging.
package gateway

import (
	"context"
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

type Gateway struct {
	cfg       *config.Config
	boxes     *box.Manager
	authStore *auth.Store

	mu        sync.Mutex
	sess      map[string]*session // cookieValue -> session
	userBoxes map[string]string   // username -> boxID
	broadcast BroadcastConfig

	cliMu     sync.Mutex
	activeCLI map[string]*websocket.Conn // sessKey -> current active CLI connection

	guiMu     sync.Mutex
	activeGUI map[string]*websocket.Conn // boxID -> current active GUI connection

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

func (g *Gateway) Run(ctx context.Context) {
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
					if s.BoxID != "" {
						if b := g.cfg.BoxByID(s.BoxID); b != nil {
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

func (g *Gateway) boxFor(r *http.Request) (*config.Box, *session) {
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
	b, s := g.boxFor(r)
	if b == nil {
		http.Error(w, "environment not running", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	_ = g.boxes.Ensure(ctx, b)

	g.renderTemplate(w, "term.html", map[string]any{"User": s.User, "Box": b})
}

func (g *Gateway) guiPage(w http.ResponseWriter, r *http.Request) {
	b, s := g.boxFor(r)
	if b == nil {
		http.Error(w, "environment not running", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	_ = g.boxes.EnsureGUI(ctx, b)

	g.renderTemplate(w, "gui.html", map[string]any{"User": s.User, "Box": b})
}
