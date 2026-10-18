// Package gateway provides the single-port HTTP/WebSocket gateway: authentication, routing, and terminal/desktop bridging.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/internal/auth"
	"attackbox/internal/box"
	"attackbox/internal/config"
)

var ErrNoAvailableBox = errors.New("no available sandbox slot")

var appLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func sanitizeLogArg(v any) any {
	if s, ok := v.(string); ok {
		s = strings.ReplaceAll(s, "\r", " ")
		s = strings.ReplaceAll(s, "\n", " ")
		return s
	}
	if err, ok := v.(error); ok {
		s := err.Error()
		s = strings.ReplaceAll(s, "\r", " ")
		s = strings.ReplaceAll(s, "\n", " ")
		return s
	}
	return v
}

func sanitizeArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = sanitizeLogArg(a)
	}
	return out
}

func logAuth(format string, v ...any) {
	appLogger.Info(fmt.Sprintf(format, sanitizeArgs(v)...), "subsystem", "AUTH")
}
func logEnv(format string, v ...any) {
	appLogger.Info(fmt.Sprintf(format, sanitizeArgs(v)...), "subsystem", "ENV")
}
func logTerm(format string, v ...any) {
	appLogger.Info(fmt.Sprintf(format, sanitizeArgs(v)...), "subsystem", "TERM")
}
func logGUI(format string, v ...any) {
	appLogger.Info(fmt.Sprintf(format, sanitizeArgs(v)...), "subsystem", "GUI")
}
func logSec(format string, v ...any) {
	appLogger.Warn(fmt.Sprintf(format, sanitizeArgs(v)...), "subsystem", "SECURITY")
}

func (g *Gateway) getClientIP(r *http.Request) string {
	if g.cfg.TrustProxy {
		if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
			return cfIP
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (g *Gateway) recordAudit(eventType, actor, target, ip, details string) {
	appLogger.Info("security audit",
		"subsystem", "AUDIT",
		"event_type", eventType,
		"actor", actor,
		"target", target,
		"ip", ip,
		"details", details,
	)
	if g.authStore != nil {
		if err := g.authStore.RecordAudit(eventType, actor, target, ip, details); err != nil {
			appLogger.Error("failed to record audit to database", "err", err)
		}
	}
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		attempts: make(map[string][]time.Time),
	}
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	var valid []time.Time
	for _, t := range l.attempts[key] {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		delete(l.attempts, key)
		return true
	}
	l.attempts[key] = valid
	return len(valid) < 5
}

func (l *loginLimiter) recordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[key] = append(l.attempts[key], time.Now())
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// BoxLease 沙箱租约生命周期，与用户账号绑定，完全独立于单端 HTTP 会话
type BoxLease struct {
	BoxID     string
	User      string
	StartedAt time.Time
	ExpiresAt time.Time
}

type session struct {
	User    string
	Role    string
	Expires time.Time // 纯用户登录凭据有效期 (如 24h)
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
	limiter   *loginLimiter

	mu          sync.Mutex
	sess        map[string]*session   // cookieValue -> session (纯认证管理)
	userLeases  map[string]*BoxLease  // username -> BoxLease (纯沙箱生命周期管理)
	boxActivity map[string]time.Time  // boxID -> last user input time (方案 A: 空闲自动释放)
	broadcast   BroadcastConfig

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

	broadcast := BroadcastConfig{
		Enabled: true,
		Message: "欢迎使用 AttackBox 靶场工作台，实验结束后请随手关闭不再使用的环境。",
	}

	// 启动时从 SQLite 恢复管理员持久化策略 (TTL/MaxSlots/IdleTimeout/Broadcast)
	if settings, err := store.GetAllSettings(); err == nil {
		if v, ok := settings["max_slots"]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
				c.MaxSlots = n
				c.EnsureSlots(n)
			}
		}
		if v, ok := settings["ttl_minutes"]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1440 {
				c.TTLMinutes = n
			}
		}
		if v, ok := settings["idle_timeout_minutes"]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 120 {
				c.IdleTimeoutMinutes = n
			}
		}
		if v, ok := settings["broadcast_enabled"]; ok {
			broadcast.Enabled = (v == "true")
		}
		if v, ok := settings["broadcast_message"]; ok && v != "" {
			broadcast.Message = v
		}
	}

	sessMap := map[string]*session{}
	if records, err := store.GetActiveSessions(); err == nil {
		now := time.Now()
		for _, rec := range records {
			if rec.ExpiresAt.After(now) {
				sessMap[rec.Token] = &session{
					User:    rec.Username,
					Role:    rec.Role,
					Expires: rec.ExpiresAt,
				}
			} else {
				_ = store.DeleteSession(rec.Token)
			}
		}
	}

	return &Gateway{
		cfg:         c,
		boxes:       m,
		authStore:   store,
		limiter:     newLoginLimiter(),
		sess:        sessMap,
		userLeases:  map[string]*BoxLease{},
		boxActivity: map[string]time.Time{},
		activeCLI:   map[string]*safeConn{},
		activeGUI:   map[string]*safeConn{},
		broadcast:   broadcast,
		novnc:       http.FileServer(http.Dir(c.NovncDir)),
		xterm:       http.FileServer(http.Dir(c.XtermDir)),
	}, nil
}

// touchBoxActivity 记录/刷新用户与沙箱的最近一次有效交互时间（敲击键盘、输入命令、鼠标点击移动）
func (g *Gateway) touchBoxActivity(boxID string) {
	if boxID == "" {
		return
	}
	g.mu.Lock()
	if g.boxActivity == nil {
		g.boxActivity = make(map[string]time.Time)
	}
	g.boxActivity[boxID] = time.Now()
	g.mu.Unlock()
}

// CleanBoot 纯净开机自愈：清退宿主机上所有可能残留的历史容器、网络命名空间、快照与运行时文件
func (g *Gateway) CleanBoot(ctx context.Context) {
	logEnv("clean boot: initiating full state purge and sandbox cleanup")
	g.boxes.CleanAll(ctx, g.cfg.Boxes)
	logEnv("clean boot: all sandbox slots, netns, and temporary files cleaned")
}

// Run 定时扫描会话与沙箱 TTL：两层完全解耦，锁内快速摘除，锁外异步 Kick I/O
func (g *Gateway) Run(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	tickCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tickCount++
			if tickCount%100 == 0 && g.authStore != nil {
				_ = g.authStore.PruneAuditLogs(50000)
			}

			now := time.Now()
			var toKick []*config.Box
			var toCloseBoxIDs []string
			var expiredTokens []string

			// 第一阶段：原子快速内存状态更新 (微秒级立即释放锁)
			g.mu.Lock()
			// 1. 清理过期认证凭据 (纯认证，不影响沙箱运行)
			for tok, s := range g.sess {
				if now.After(s.Expires) {
					logAuth("session expired: user=%s", s.User)
					delete(g.sess, tok)
					expiredTokens = append(expiredTokens, tok)
				}
			}

			// 2. 扫描独立沙箱租约到期及无交互空闲超时情况 (方案 A: 10分钟无交互自动释放)
			idleLimit := time.Duration(g.cfg.IdleTimeoutMinutes) * time.Minute
			if idleLimit <= 0 {
				idleLimit = 10 * time.Minute
			}

			for username, lease := range g.userLeases {
				lastAct, hasAct := g.boxActivity[lease.BoxID]
				if !hasAct {
					lastAct = lease.StartedAt
					g.boxActivity[lease.BoxID] = lastAct
				}

				isTTLExpired := now.After(lease.ExpiresAt)
				isIdleExpired := now.Sub(lastAct) >= idleLimit

				if isTTLExpired || isIdleExpired {
					reason := "ttl expired"
					if isIdleExpired {
						reason = fmt.Sprintf("idle timeout (%v without user interaction)", idleLimit)
					}
					logEnv("sandbox expired: user=%s box=%s reason=%s last_active=%s",
						username, lease.BoxID, reason, lastAct.Format("15:04:05"))
					if b := g.cfg.BoxByID(lease.BoxID); b != nil {
						toKick = append(toKick, b)
						toCloseBoxIDs = append(toCloseBoxIDs, lease.BoxID)
					}
					delete(g.userLeases, username)
					delete(g.boxActivity, lease.BoxID)
				}
			}
			g.mu.Unlock() // 立即释放全局锁，保证前台所有用户 HTTP API 绝对不卡顿

			// 第二阶段：在锁外执行慢速 I/O 与连接切断
			for _, tok := range expiredTokens {
				_ = g.authStore.DeleteSession(tok)
			}
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

	// 公开系统策略
	mux.HandleFunc("GET /api/public/config", g.apiPublicConfig)

	// 页面路由
	mux.HandleFunc("GET /{$}", g.indexPage)
	mux.HandleFunc("GET /login", g.indexPage)
	mux.HandleFunc("GET /workspace", g.indexPage)
	mux.HandleFunc("GET /account", g.indexPage)
	mux.HandleFunc("GET /logout", g.logout)
	mux.HandleFunc("GET /term", g.auth(g.termPage))
	mux.HandleFunc("GET /gui", g.auth(g.guiPage))
	mux.HandleFunc("GET /admin", g.adminAuth(g.adminPage))

	// REST API: 认证
	mux.HandleFunc("POST /api/login", g.apiLogin)
	mux.HandleFunc("POST /api/register", g.apiRegister)
	mux.HandleFunc("POST /api/auth/send-verification-code", g.apiSendVerificationCode)
	mux.HandleFunc("POST /api/logout", g.apiLogout)
	mux.HandleFunc("GET /api/me", g.apiMe)
	mux.HandleFunc("POST /api/user/password", g.auth(g.apiUserChangePassword))

	// REST API: OAuth 第三方认证与两步注册
	mux.HandleFunc("GET /api/public/oauth-providers", g.apiPublicOAuthProviders)
	mux.HandleFunc("GET /api/oauth/{provider}/authorize", g.apiOAuthAuthorize)
	mux.HandleFunc("GET /api/oauth/{provider}/callback", g.apiOAuthCallback)
	mux.HandleFunc("POST /api/oauth/complete-registration", g.apiOAuthCompleteRegistration)
	mux.HandleFunc("GET /api/admin/oauth/settings", g.adminAuth(g.apiAdminOAuthGetSettings))
	mux.HandleFunc("POST /api/admin/oauth/settings", g.adminAuth(g.apiAdminOAuthUpdateSettings))

	// REST API: Resend 邮件分发服务 (零落地密钥保管库)
	mux.HandleFunc("GET /api/admin/resend/settings", g.adminAuth(g.apiAdminResendGetSettings))
	mux.HandleFunc("POST /api/admin/resend/settings", g.adminAuth(g.apiAdminResendUpdateSettings))

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
	mux.HandleFunc("GET /api/admin/sandboxes", g.adminAuth(g.apiAdminListSandboxes))
	mux.HandleFunc("POST /api/admin/sandboxes/release", g.adminAuth(g.apiAdminReleaseSandbox))
	mux.HandleFunc("GET /api/admin/audit", g.adminAuth(g.apiAdminListAudit))
	mux.HandleFunc("GET /api/admin/settings", g.adminAuth(g.apiAdminGetSettings))
	mux.HandleFunc("POST /api/admin/settings", g.adminAuth(g.apiAdminUpdateSettings))

	// 静态资源与核心库 (直接映射磁盘目录，改动后 F5 立即生效)
	staticDir := filepath.Join(g.cfg.WebDir, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	distAssetsDir := filepath.Join(g.cfg.WebDir, "dist", "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(distAssetsDir))))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "favicon.ico"))
	})
	mux.HandleFunc("GET /logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "logo.png"))
	})
	mux.Handle("GET /xterm/", http.StripPrefix("/xterm/", g.xterm))
	mux.Handle("GET /vnc/", http.StripPrefix("/vnc/", g.novnc))

	// WebSocket 桥接
	mux.HandleFunc("GET /term/ws", g.auth(g.termWS))
	mux.HandleFunc("GET /vnc/ws", g.auth(g.vncWS))

	return secureHeaders(mux)
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// ---------- 鉴权中间件 ----------

func (g *Gateway) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := g.lookup(r)
		if s == nil {
			if strings.HasSuffix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
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
			// 404 伪装：对外彻底伪装端点或页面不存在，杜绝指纹识别与枚举
			http.NotFound(w, r)
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

// currentBoxFor 纯只读查询用户当前沙箱租约，绝不擅自分配槽位 (供 apiMe 等状态检测使用，彻底解决 Bug B)
func (g *Gateway) currentBoxFor(r *http.Request) (*config.Box, *session) {
	s := g.lookup(r)
	if s == nil {
		return nil, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	lease := g.userLeases[s.User]
	if lease == nil || lease.BoxID == "" {
		return nil, s
	}
	return g.cfg.BoxByID(lease.BoxID), s
}

// acquireBoxFor 按需独占申请沙箱租约 (仅在明确启动/访问环境时调用，彻底杜绝 Bug A 夺舍与 Bug B 虚占)
func (g *Gateway) acquireBoxFor(r *http.Request) (*config.Box, *session, error) {
	s := g.lookup(r)
	if s == nil {
		return nil, nil, errors.New("unauthorized")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if lease, exists := g.userLeases[s.User]; exists && lease != nil {
		b := g.cfg.BoxByID(lease.BoxID)
		if b != nil {
			g.boxActivity[b.ID] = time.Now()
			return b, s, nil
		}
	}

	// 校验当前活跃租约总数是否已达 MaxSlots
	if len(g.userLeases) >= g.cfg.MaxSlots {
		return nil, s, ErrNoAvailableBox
	}

	// 查找当前真正空闲（未被任何用户租用）的槽位，且槽位编号不超过 MaxSlots
	inUse := make(map[string]bool)
	for _, l := range g.userLeases {
		inUse[l.BoxID] = true
	}

	for i := range g.cfg.Boxes {
		b := &g.cfg.Boxes[i]
		if b.Index > g.cfg.MaxSlots {
			continue
		}
		bid := b.ID
		if !inUse[bid] {
			now := time.Now()
			ttl := g.cfg.TTLMinutes
			if ttl <= 0 {
				ttl = b.TTLMinutes
			}
			expires := now.Add(time.Duration(ttl) * time.Minute)
			g.userLeases[s.User] = &BoxLease{
				BoxID:     bid,
				User:      s.User,
				StartedAt: now,
				ExpiresAt: expires,
			}
			g.boxActivity[bid] = now
			logEnv("lease acquired: user=%s box=%s ttl=%dm expires=%s", s.User, bid, ttl, expires.Format("15:04:05"))
			return b, s, nil
		}
	}

	// 槽位已满，绝不回退至 box1！返回错误由调用方优雅向前端报错
	return nil, s, ErrNoAvailableBox
}

// evictUser 彻底注销指定用户的所有端会话、掐断连接并立即销毁其沙箱租约
func (g *Gateway) evictUser(ctx context.Context, username string) {
	var boxIDToKick string
	var boxToKick *config.Box

	g.mu.Lock()
	for tok, s := range g.sess {
		if s.User == username {
			delete(g.sess, tok)
		}
	}
	if lease, exists := g.userLeases[username]; exists {
		boxIDToKick = lease.BoxID
		boxToKick = g.cfg.BoxByID(lease.BoxID)
		delete(g.userLeases, username)
		delete(g.boxActivity, boxIDToKick)
	}
	g.mu.Unlock()

	_ = g.authStore.DeleteSessionsByUsername(username)

	if boxIDToKick != "" {
		logEnv("evict user: user=%s box=%s", username, boxIDToKick)
		g.closeBoxSessions(boxIDToKick)
		if boxToKick != nil {
			_ = g.boxes.Kick(ctx, boxToKick)
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
	distIndex := filepath.Join(g.cfg.WebDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeFile(w, r, distIndex)
		return
	}
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

	distIndex := filepath.Join(g.cfg.WebDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeFile(w, r, distIndex)
		return
	}

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

	distIndex := filepath.Join(g.cfg.WebDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeFile(w, r, distIndex)
		return
	}

	g.renderTemplate(w, "gui.html", map[string]any{"User": s.User, "Box": b})
}

func (g *Gateway) adminPage(w http.ResponseWriter, r *http.Request) {
	distAdmin := filepath.Join(g.cfg.WebDir, "dist", "admin.html")
	if _, err := os.Stat(distAdmin); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeFile(w, r, distAdmin)
		return
	}
	http.NotFound(w, r)
}
