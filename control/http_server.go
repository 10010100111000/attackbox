package control

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"sync"
)

type ControlHTTPServer struct {
	cfg        *ControlConfig
	db         *DatabaseStore
	privKey    ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	clusterMgr *NodeClusterManager
	slots      *SlotAllocator
	sessions   *SessionManager
	handler    http.Handler
	cleanupMu  sync.Mutex
	cleaning   map[string]bool
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

	s := &ControlHTTPServer{
		cfg:        cfg,
		db:         db,
		privKey:    privKey,
		pubKey:     pubKey,
		clusterMgr: clusterMgr,
		slots:      slots,
		sessions:   sessions,
		cleaning:   make(map[string]bool),
	}

	if clusterMgr != nil {
		clusterMgr.SetHeartbeatHandler(s.syncWorkerState)
	}

	// Bootstrap admin user if specified in config
	if cfg.Auth.InitialAdminUsername != "" && cfg.Auth.InitialAdminPassword != "" && db != nil {
		adminHash, err := HashPassword(cfg.Auth.InitialAdminPassword)
		if err == nil {
			_ = db.EnsureAdminUser(cfg.Auth.InitialAdminUsername, adminHash, cfg.Auth.InitialAdminEmail)
		}
	}

	mux := http.NewServeMux()

	// 0. Health probe
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})

	// 1. Worker Reverse WebSocket Agent endpoint
	mux.HandleFunc("/api/agent/connect", s.clusterMgr.HandleAgentConnect)

	// 2. Public Authentication endpoints
	mux.HandleFunc("/api/auth/register", s.handleRegister)
	mux.HandleFunc("/api/auth/login", s.handleLogin)

	// 3. Protected User & Sandbox endpoints
	authMw := s.authMiddleware
	mux.Handle("/api/user/me", authMw(http.HandlerFunc(s.handleMe)))
	mux.Handle("/api/sandbox/start", authMw(http.HandlerFunc(s.handleStartSandbox)))
	mux.Handle("/api/sandbox/stop", authMw(http.HandlerFunc(s.handleStopSandbox)))
	mux.Handle("/api/sandbox/active", authMw(http.HandlerFunc(s.handleActiveSandbox)))

	// Wrap in global middlewares (Recovery -> RequestID -> Logging -> CORS -> BodyLimit)
	handler := s.bodyLimitMiddleware(mux)
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
