package control

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
)

type ControlHTTPServer struct {
	cfg         *ControlConfig
	db          *DatabaseStore
	privKey     ed25519.PrivateKey
	pubKey      ed25519.PublicKey
	clusterMgr  *NodeClusterManager
	slots       *SlotAllocator
	sessions    *SessionManager
	webSessions *scs.SessionManager
	handler     http.Handler
	cleanupMu   sync.Mutex
	cleaning    map[string]bool
}

func (s *ControlHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.handler != nil {
		s.handler.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}

// NewControlHTTPServer instantiates the complete HTTP API server with all middlewares.
func NewControlHTTPServer(
	cfg *ControlConfig,
	db *DatabaseStore,
	privKey ed25519.PrivateKey,
	pubKey ed25519.PublicKey,
	clusterMgr *NodeClusterManager,
	slots *SlotAllocator,
	sessions *SessionManager,
) http.Handler {
	if cfg == nil {
		cfg = &ControlConfig{}
	}
	cfg.SanitizeAndApplyDefaults()

	if slots == nil {
		slots = NewSlotAllocator(250)
	}
	if sessions == nil {
		sessions = NewSessionManager()
	}

	// Initialize universal Web Session Manager via scs/v2
	webSessions := scs.New()
	webSessions.Lifetime = 24 * time.Hour
	webSessions.Cookie.Name = "attackbox_session"
	webSessions.Cookie.HttpOnly = true
	webSessions.Cookie.SameSite = http.SameSiteLaxMode
	webSessions.Cookie.Path = "/"
	webSessions.Cookie.Secure = false // Allowed for current HTTP/IP setup; enabled via reverse proxy/TLS

	s := &ControlHTTPServer{
		cfg:         cfg,
		db:          db,
		privKey:     privKey,
		pubKey:      pubKey,
		clusterMgr:  clusterMgr,
		slots:       slots,
		sessions:    sessions,
		webSessions: webSessions,
		cleaning:    make(map[string]bool),
	}

	if clusterMgr != nil {
		clusterMgr.SetHeartbeatHandler(s.syncWorkerState)
	}

	// Bootstrap initial administrator user if configured
	if cfg.Auth.InitialAdminUsername != "" && cfg.Auth.InitialAdminPassword != "" && db != nil {
		adminHash, err := HashPassword(cfg.Auth.InitialAdminPassword)
		if err == nil {
			_ = db.EnsureAdminUser(cfg.Auth.InitialAdminUsername, adminHash, cfg.Auth.InitialAdminEmail)
		}
	}

	mux := http.NewServeMux()

	// 0. Health probe & public configuration
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/public/config", s.handlePublicConfig)

	// 1. Worker Reverse WebSocket Agent endpoint
	mux.HandleFunc("/api/agent/connect", s.clusterMgr.HandleAgentConnect)

	// 2. Authentication & Verification endpoints
	mux.HandleFunc("/api/auth/send-verification-code", s.handleSendVerificationCode)
	mux.HandleFunc("/api/auth/send-code", s.handleSendVerificationCode)
	mux.HandleFunc("/api/auth/register", s.handleRegister)
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)

	// 3. Protected User & Sandbox endpoints
	authMw := s.authMiddleware
	mux.Handle("/api/user/me", authMw(http.HandlerFunc(s.handleMe)))
	mux.Handle("/api/me", authMw(http.HandlerFunc(s.handleMe)))
	mux.Handle("/api/user/username", authMw(http.HandlerFunc(s.handleUpdateUsername)))
	mux.Handle("/api/sandbox/start", authMw(http.HandlerFunc(s.handleStartSandbox)))
	mux.Handle("/api/sandbox/stop", authMw(http.HandlerFunc(s.handleStopSandbox)))
	mux.Handle("/api/sandbox/active", authMw(http.HandlerFunc(s.handleActiveSandbox)))
	mux.Handle("/api/env/cli/toggle", authMw(http.HandlerFunc(s.handleStartSandbox)))
	mux.Handle("/api/env/gui/toggle", authMw(http.HandlerFunc(s.handleGuiToggle)))
	mux.Handle("/api/env/destroy", authMw(http.HandlerFunc(s.handleStopSandbox)))

	// Wrap in global middlewares (Recovery -> RequestID -> Logging -> CORS -> Session -> BodyLimit)
	handler := s.bodyLimitMiddleware(mux)
	handler = s.webSessions.LoadAndSave(handler)
	handler = s.corsMiddleware(s.cfg.Server.CORSAllowedOrigins)(handler)
	handler = s.loggingMiddleware(handler)
	handler = s.requestIDMiddleware(handler)
	handler = s.recoveryMiddleware(handler)

	s.handler = handler
	return s
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]interface{}{
		"error":   message,
		"code":    statusCode,
		"success": false,
	})
}
