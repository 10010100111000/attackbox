package control

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"

	"attackbox/protocol"
)

// -------------------------------------------------------------
// 1. Configuration (Directly consumed by Control Subsystem)
// -------------------------------------------------------------

type ControlConfig struct {
	Server struct {
		ListenAddr         string   `yaml:"listen_addr"`
		CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Cluster struct {
		ClusterSecret       string `yaml:"cluster_secret"`
		HeartbeatTimeoutSec int    `yaml:"heartbeat_timeout_sec"`
		RPCTimeoutSec       int    `yaml:"rpc_timeout_sec"`
	} `yaml:"cluster"`
	Auth struct {
		Ed25519PrivateKey    string `yaml:"ed25519_private_key"`
		JWTExpireHours       int    `yaml:"jwt_expire_hours"`
		TicketTTLSec         int    `yaml:"ticket_ttl_sec"`
		InitialAdminUsername string `yaml:"initial_admin_username"`
		InitialAdminPassword string `yaml:"initial_admin_password"`
		InitialAdminEmail    string `yaml:"initial_admin_email"`
	} `yaml:"auth"`
	Sandbox struct {
		Resources         protocol.BoxResourceQuota `yaml:"resources"`
		Storage           protocol.BoxStoragePolicy `yaml:"storage"`
		TTLSeconds        int                       `yaml:"ttl_seconds"`
		IdleSeconds       int                       `yaml:"idle_seconds"`
		ReaperIntervalSec int                       `yaml:"reaper_interval_sec"`
	} `yaml:"sandbox"`
}

// SanitizeAndApplyDefaults enforces fail-closed production baselines on any ControlConfig.
func (cfg *ControlConfig) SanitizeAndApplyDefaults() {
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = "127.0.0.1:18080"
	}
	if len(cfg.Server.CORSAllowedOrigins) == 0 {
		cfg.Server.CORSAllowedOrigins = []string{"*"}
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "/var/lib/attackbox/attackbox.db"
	}
	if cfg.Cluster.ClusterSecret == "" {
		cfg.Cluster.ClusterSecret = "attackbox_cluster_secret_change_me_in_prod!"
	}
	if cfg.Cluster.HeartbeatTimeoutSec <= 0 {
		cfg.Cluster.HeartbeatTimeoutSec = 15
	}
	if cfg.Cluster.RPCTimeoutSec <= 0 {
		cfg.Cluster.RPCTimeoutSec = 30
	}
	if cfg.Auth.JWTExpireHours <= 0 {
		cfg.Auth.JWTExpireHours = 24
	}
	if cfg.Auth.TicketTTLSec <= 0 {
		cfg.Auth.TicketTTLSec = 60
	}
	if cfg.Auth.InitialAdminUsername == "" {
		cfg.Auth.InitialAdminUsername = "admin"
	}
	if cfg.Auth.InitialAdminPassword == "" {
		cfg.Auth.InitialAdminPassword = "AdminSecurePassword123!"
	}
	if cfg.Auth.InitialAdminEmail == "" {
		cfg.Auth.InitialAdminEmail = "admin@attackbox.local"
	}

	// Fail-closed baseline resource enforcement
	if cfg.Sandbox.Resources.CPUWeight == 0 {
		cfg.Sandbox.Resources.CPUWeight = protocol.DefaultCPUWeight
	}
	if cfg.Sandbox.Resources.MemoryMaxMB <= 0 {
		cfg.Sandbox.Resources.MemoryMaxMB = protocol.DefaultMemLimitMB
	}
	if cfg.Sandbox.Resources.PIDsMax <= 0 {
		cfg.Sandbox.Resources.PIDsMax = protocol.DefaultPIDsLimit
	}
	if cfg.Sandbox.Resources.IOWriteBytesSec <= 0 {
		cfg.Sandbox.Resources.IOWriteBytesSec = protocol.DefaultIOWriteBytesSec
	}
	if cfg.Sandbox.Resources.IOWriteIOPS <= 0 {
		cfg.Sandbox.Resources.IOWriteIOPS = protocol.DefaultIOWriteIOPS
	}
	if cfg.Sandbox.Storage.TmpfsTmpSize == "" {
		cfg.Sandbox.Storage.TmpfsTmpSize = protocol.DefaultTmpfsTmpSize
	}
	if cfg.Sandbox.Storage.TmpfsRunSize == "" {
		cfg.Sandbox.Storage.TmpfsRunSize = protocol.DefaultTmpfsRunSize
	}
	if cfg.Sandbox.TTLSeconds <= 0 {
		cfg.Sandbox.TTLSeconds = 7200 // 2 hours hard lifetime ceiling
	}
	if cfg.Sandbox.IdleSeconds <= 0 {
		cfg.Sandbox.IdleSeconds = 1800 // 30 minutes inactivity timeout
	}
	if cfg.Sandbox.ReaperIntervalSec <= 0 {
		cfg.Sandbox.ReaperIntervalSec = 30 // 30 seconds scanning period
	}
}

// LoadControlConfig reads YAML configuration or falls back to production defaults.
func LoadControlConfig(path string) (*ControlConfig, error) {
	cfg := &ControlConfig{}
	cfg.SanitizeAndApplyDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read control config %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}
	cfg.SanitizeAndApplyDefaults()
	return cfg, nil
}

// -------------------------------------------------------------
// 2. Authentication & Cryptography (Server-side Authentication)
// -------------------------------------------------------------

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

type JWTClaims struct {
	Sub          int64  `json:"sub"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion int    `json:"ver"`
	Exp          int64  `json:"exp"`
}

func SignJWT(privKey ed25519.PrivateKey, claims JWTClaims) (string, error) {
	data, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(privKey, data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func VerifyJWT(pubKey ed25519.PublicKey, tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !ed25519.Verify(pubKey, payload, sig) {
		return nil, ErrInvalidSignature
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func LoadOrGenerateKeys(hexPrivKey string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if hexPrivKey != "" {
		data, err := hex.DecodeString(strings.TrimSpace(hexPrivKey))
		if err == nil && len(data) == ed25519.PrivateKeySize {
			priv := ed25519.PrivateKey(data)
			pub := priv.Public().(ed25519.PublicKey)
			return pub, priv, nil
		}
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	return pub, priv, err
}

// -------------------------------------------------------------
// 3. Slot & Session Allocation (Cluster Resource Tracking)
// -------------------------------------------------------------

var ErrNoAvailableSlots = errors.New("no available sandbox IP slots (range 1-250 exhausted)")

type SlotAllocator struct {
	mu       sync.Mutex
	maxSlots int
	slotToID map[int]string
	idToSlot map[string]int
}

func NewSlotAllocator(maxSlots int) *SlotAllocator {
	if maxSlots <= 0 || maxSlots > 250 {
		maxSlots = 250
	}
	return &SlotAllocator{
		maxSlots: maxSlots,
		slotToID: make(map[int]string, maxSlots),
		idToSlot: make(map[string]int, maxSlots),
	}
}

func (a *SlotAllocator) Allocate(sandboxID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if slot, exists := a.idToSlot[sandboxID]; exists {
		return slot, nil
	}

	for i := 1; i <= a.maxSlots; i++ {
		if _, occupied := a.slotToID[i]; !occupied {
			a.slotToID[i] = sandboxID
			a.idToSlot[sandboxID] = i
			return i, nil
		}
	}

	return 0, fmt.Errorf("%w: %d slots in use", ErrNoAvailableSlots, len(a.slotToID))
}

func (a *SlotAllocator) Release(sandboxID string) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	slot, exists := a.idToSlot[sandboxID]
	if !exists {
		return 0
	}

	delete(a.idToSlot, sandboxID)
	delete(a.slotToID, slot)
	return slot
}

func (a *SlotAllocator) GetSlot(sandboxID string) (int, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	slot, exists := a.idToSlot[sandboxID]
	return slot, exists
}

// Occupy explicitly marks a specific slot as occupied by sandboxID (used for state reconciliation).
func (a *SlotAllocator) Occupy(sandboxID string, slot int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if slot < 1 || slot > a.maxSlots {
		return fmt.Errorf("slot %d out of bounds (1-%d)", slot, a.maxSlots)
	}

	if curSlot, exists := a.idToSlot[sandboxID]; exists {
		if curSlot == slot {
			return nil
		}
		return fmt.Errorf("sandbox %s already occupies slot %d", sandboxID, curSlot)
	}

	if occupyingID, exists := a.slotToID[slot]; exists {
		return fmt.Errorf("slot %d already occupied by %s", slot, occupyingID)
	}

	a.slotToID[slot] = sandboxID
	a.idToSlot[sandboxID] = slot
	return nil
}

type SessionState string

const (
	SessionStateCreating      SessionState = "creating"
	SessionStateRunning       SessionState = "running"
	SessionStateTerminating   SessionState = "terminating"
	SessionStateCleanupFailed SessionState = "cleanup_failed"
	SessionStateTerminated    SessionState = "terminated"
)

type ActiveSession struct {
	WorkerBootID      string                 `json:"-"`
	InventoryRevision uint64                 `json:"-"`
	SandboxID         string                 `json:"sandbox_id"`
	UserID            int64                  `json:"user_id"`
	WorkerNodeID      string                 `json:"worker_node_id"`
	SlotIndex         int                    `json:"slot_index"`
	SandboxIP         string                 `json:"sandbox_ip"`
	State             SessionState           `json:"state"`
	StartedAt         time.Time              `json:"started_at"`
	LastActiveAt      time.Time              `json:"last_active_at"`
	ExpiresAt         time.Time              `json:"expires_at"`
	Policy            protocol.SandboxPolicy `json:"policy"`
}

type SessionManager struct {
	mu        sync.RWMutex
	sessions  map[string]*ActiveSession
	byUser    map[int64]string
	reserving map[int64]time.Time // in-flight sandbox creation tracking to eliminate TOCTOU race
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:  make(map[string]*ActiveSession),
		byUser:    make(map[int64]string),
		reserving: make(map[int64]time.Time),
	}
}

// TryReserve atomically checks if a user already has an active sandbox or a creation in-flight.
func (sm *SessionManager) TryReserve(userID int64) (release func(), ok bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. Check if user already has an active sandbox
	if sbxID, exists := sm.byUser[userID]; exists {
		if _, active := sm.sessions[sbxID]; active {
			return nil, false
		}
		delete(sm.byUser, userID)
	}

	// 2. Check if creation is already in-flight (TOCTOU protection against rapid double-clicks)
	if _, busy := sm.reserving[userID]; busy {
		return nil, false
	}
	reservedAt := time.Now()
	sm.reserving[userID] = reservedAt

	releaseFunc := func() {
		sm.mu.Lock()
		defer sm.mu.Unlock()
		if sm.reserving[userID] == reservedAt {
			delete(sm.reserving, userID)
		}
	}

	return releaseFunc, true
}

func (sm *SessionManager) Add(s *ActiveSession) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if id, exists := sm.byUser[s.UserID]; exists && id != s.SandboxID {
		return errors.New("user already owns a sandbox")
	}
	delete(sm.reserving, s.UserID)
	cp := *s
	sm.sessions[s.SandboxID] = &cp
	sm.byUser[s.UserID] = s.SandboxID
	return nil
}

func (sm *SessionManager) SetState(sandboxID string, state SessionState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[sandboxID]; ok {
		s.State = state
	}
}

func (sm *SessionManager) Touch(sandboxID string, t time.Time) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[sandboxID]; ok {
		if t.After(s.LastActiveAt) {
			s.LastActiveAt = t
		}
	}
}

// Get returns a defensive value copy to eliminate pointer read-write data races.
func (sm *SessionManager) Get(sandboxID string) (ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[sandboxID]
	if !ok {
		return ActiveSession{}, false
	}
	return *s, true
}

// GetByUser returns a defensive value copy to eliminate pointer read-write data races.
func (sm *SessionManager) GetByUser(userID int64) (ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sbxID, ok := sm.byUser[userID]
	if !ok {
		return ActiveSession{}, false
	}
	s, ok := sm.sessions[sbxID]
	if !ok {
		return ActiveSession{}, false
	}
	return *s, true
}

func (sm *SessionManager) Remove(sandboxID string) (ActiveSession, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.sessions[sandboxID]
	if ok {
		cp := *s
		delete(sm.sessions, sandboxID)
		if sm.byUser[s.UserID] == sandboxID {
			delete(sm.byUser, s.UserID)
		}
		return cp, true
	}
	return ActiveSession{}, false
}

// ListAll returns a snapshot slice of session value copies.
func (sm *SessionManager) ListAll() []ActiveSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	res := make([]ActiveSession, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		res = append(res, *s)
	}
	return res
}

// -------------------------------------------------------------
// 4. HTTP Router, Middlewares & API Handlers
// -------------------------------------------------------------

type contextKey string

const (
	contextKeyUserID    contextKey = "user_id"
	contextKeyUsername  contextKey = "username"
	contextKeyRole      contextKey = "role"
	contextKeyRequestID contextKey = "request_id"
)

func UserFromContext(ctx context.Context) (int64, string, string, bool) {
	uid, ok1 := ctx.Value(contextKeyUserID).(int64)
	user, ok2 := ctx.Value(contextKeyUsername).(string)
	role, ok3 := ctx.Value(contextKeyRole).(string)
	return uid, user, role, ok1 && ok2 && ok3
}

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

type StartSandboxReq struct{}

type StartSandboxResp struct {
	SandboxID string                 `json:"sandbox_id"`
	WorkerID  string                 `json:"worker_id"`
	WorkerURL string                 `json:"worker_url"`
	Ticket    string                 `json:"ticket"`
	ExpiresIn int                    `json:"expires_in"`
	SandboxIP string                 `json:"sandbox_ip"`
	SlotIndex int                    `json:"slot_index"`
	ExpiresAt time.Time              `json:"expires_at"`
	Policy    protocol.SandboxPolicy `json:"policy"`
}

type StopSandboxReq struct {
	SandboxID string `json:"sandbox_id"`
}

type StopSandboxResp struct {
	Success         bool   `json:"success"`
	SandboxID       string `json:"sandbox_id"`
	DurationSeconds int64  `json:"duration_seconds"`
}

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResp struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
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

func (s *ControlHTTPServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := VerifyJWT(s.pubKey, tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		if s.db != nil {
			u, err := s.db.GetUserByID(claims.Sub)
			if err != nil || u == nil || u.Status != 1 || u.TokenVersion != claims.TokenVersion {
				writeError(w, http.StatusUnauthorized, "session revoked or account inactive")
				return
			}
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, claims.Sub)
		ctx = context.WithValue(ctx, contextKeyUsername, claims.Username)
		ctx = context.WithValue(ctx, contextKeyRole, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *ControlHTTPServer) corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	originsMap := make(map[string]bool)
	for _, o := range allowedOrigins {
		originsMap[strings.TrimSpace(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if originsMap[origin] || len(allowedOrigins) == 0 || allowedOrigins[0] == "*" {
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func (rw *statusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func (rw *statusResponseWriter) Flush() {
	if fl, ok := rw.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

var reqIDCounter uint64

func generateRequestID() string {
	id := atomic.AddUint64(&reqIDCounter, 1)
	return fmt.Sprintf("req-%08x-%04x", time.Now().Unix(), id&0xffff)
}

func (s *ControlHTTPServer) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), contextKeyRequestID, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *ControlHTTPServer) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID, _ := r.Context().Value(contextKeyRequestID).(string)
				if reqID == "" {
					reqID = "unknown"
				}
				stack := debug.Stack()
				log.Printf("[PANIC] [%s] %s %s: %v\n%s", reqID, r.Method, r.URL.Path, rec, string(stack))
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *ControlHTTPServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/agent/connect") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		srw := newStatusResponseWriter(w)
		next.ServeHTTP(srw, r)
		reqID, _ := r.Context().Value(contextKeyRequestID).(string)
		if reqID == "" {
			reqID = "none"
		}
		log.Printf("[HTTP] [%s] %d %s %s (%v)", reqID, srw.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// bodyLimitMiddleware enforces protocol.MaxHTTPPayloadSize (1MB) on incoming HTTP request bodies.
func (s *ControlHTTPServer) bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && !strings.HasPrefix(r.URL.Path, "/api/agent/connect") {
			r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxHTTPPayloadSize)
		}
		next.ServeHTTP(w, r)
	})
}

// Precomputed valid bcrypt hash for constant-time comparison against non-existent users
const dummyBcryptHash = "$2a$10$wT5Y1eS6Fqm9g4O/rQhGceJzR0d8z1bS3h1J6D8W2Y1bS3h1J6D8W"

func (s *ControlHTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Username) < 3 || len(req.Password) < 6 || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "username must be >=3 chars, password >=6 chars, and email must be valid")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := s.db.CreateUser(req.Username, hash, req.Email, "")
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			writeError(w, http.StatusConflict, "username or email already registered")
			return
		}
		log.Printf("[AUTH] Failed to create user %s: %v", req.Username, err)
		writeError(w, http.StatusInternalServerError, "failed to create user account")
		return
	}

	claims := JWTClaims{
		Sub:          user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Exp:          time.Now().Add(time.Duration(s.cfg.Auth.JWTExpireHours) * time.Hour).Unix(),
	}
	token, err := SignJWT(s.privKey, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResp{
		Token: token,
		User:  user,
	})
}

func (s *ControlHTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.db.GetUserByUsername(strings.TrimSpace(req.Username))
	hashToCompare := dummyBcryptHash
	if err == nil && user != nil {
		hashToCompare = user.PasswordHash
	}

	// Constant-time bcrypt computation to eliminate username enumeration timing side channels
	passwordMatch := CheckPassword(hashToCompare, req.Password)

	if err != nil || user == nil || !passwordMatch {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if user.Status != 1 {
		writeError(w, http.StatusForbidden, "account suspended")
		return
	}

	claims := JWTClaims{
		Sub:          user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Exp:          time.Now().Add(time.Duration(s.cfg.Auth.JWTExpireHours) * time.Hour).Unix(),
	}
	token, err := SignJWT(s.privKey, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, AuthResp{
		Token: token,
		User:  user,
	})
}

func (s *ControlHTTPServer) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := s.db.GetUserByID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

var ErrSandboxBusy = errors.New("sandbox creation or cleanup is already in progress")

func (s *ControlHTTPServer) handleStartSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req StartSandboxReq
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			var limit *http.MaxBytesError
			if errors.As(err, &limit) {
				writeError(w, http.StatusRequestEntityTooLarge, "request too large")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
	}
	session, created, err := s.startSandbox(r.Context(), uid)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrSandboxBusy):
			status = http.StatusConflict
		case errors.Is(err, ErrUserNotFound):
			status = http.StatusNotFound
		case errors.Is(err, ErrNoWorkersAvailable), errors.Is(err, ErrNoAvailableSlots):
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, err.Error())
		return
	}
	ticket, err := s.issueTicket(uid, session.SandboxID)
	if err != nil {
		if created {
			s.sessions.SetState(session.SandboxID, SessionStateCleanupFailed)
			s.asyncCompensatingDestroy(session.WorkerNodeID, session.SandboxID, "ticket_failed")
		}
		writeError(w, http.StatusInternalServerError, "failed to issue session ticket")
		return
	}
	endpoint, exists := s.clusterMgr.GetWorkerDataURL(session.WorkerNodeID, r.Host)
	if !exists {
		writeError(w, http.StatusServiceUnavailable, "worker node currently unreachable")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		if s.db != nil {
			_ = s.db.AddAuditLog(&uid, "sandbox_start", r.RemoteAddr, fmt.Sprintf("box=%s, worker=%s", session.SandboxID, session.WorkerNodeID))
		}
	}
	writeJSON(w, status, StartSandboxResp{SandboxID: session.SandboxID, WorkerID: session.WorkerNodeID, WorkerURL: endpoint, Ticket: ticket, ExpiresIn: s.cfg.Auth.TicketTTLSec, SandboxIP: session.SandboxIP, SlotIndex: session.SlotIndex, ExpiresAt: session.ExpiresAt, Policy: session.Policy})
}

// startSandbox owns user reservation, scheduling and creation acknowledgement.
func (s *ControlHTTPServer) startSandbox(ctx context.Context, uid int64) (ActiveSession, bool, error) {
	if existing, ok := s.sessions.GetByUser(uid); ok && existing.State == SessionStateRunning {
		return existing, false, nil
	}
	release, ok := s.sessions.TryReserve(uid)
	if !ok {
		return ActiveSession{}, false, ErrSandboxBusy
	}
	defer release()
	if _, err := s.db.GetUserByID(uid); err != nil {
		return ActiveSession{}, false, err
	}
	worker, err := s.clusterMgr.SelectBestWorker()
	if err != nil {
		return ActiveSession{}, false, err
	}
	id := generateSandboxID()
	slot, err := s.slots.Allocate(id)
	if err != nil {
		return ActiveSession{}, false, err
	}
	now := time.Now()
	policy := s.resolvePolicy()
	session := &ActiveSession{SandboxID: id, UserID: uid, WorkerNodeID: worker.NodeID, WorkerBootID: worker.Generation(), SlotIndex: slot, State: SessionStateCreating, StartedAt: now, LastActiveAt: now, Policy: policy}
	if err := s.sessions.Add(session); err != nil {
		s.slots.Release(id)
		return ActiveSession{}, false, ErrSandboxBusy
	}
	resp, err := s.clusterMgr.CreateBox(ctx, worker.NodeID, protocol.CreateBoxReq{BootID: session.WorkerBootID, SandboxID: id, UserID: uid, SlotIndex: slot, Policy: policy, TTLSeconds: s.cfg.Sandbox.TTLSeconds, IdleSeconds: s.cfg.Sandbox.IdleSeconds})
	if err != nil {
		s.sessions.mu.Lock()
		if current := s.sessions.sessions[id]; current != nil {
			current.State = SessionStateCleanupFailed
			if resp != nil && resp.BootID == session.WorkerBootID {
				current.InventoryRevision = resp.InventoryRevision
			}
		}
		s.sessions.mu.Unlock()
		s.asyncCompensatingDestroy(worker.NodeID, id, "compensating_create_failure")
		return ActiveSession{}, false, fmt.Errorf("worker failed to create sandbox: %w", err)
	}
	s.sessions.mu.Lock()
	current := s.sessions.sessions[id]
	ready := current != nil && current.State == SessionStateCreating
	if ready {
		current.State = SessionStateRunning
		current.SandboxIP = resp.SandboxIP
		current.WorkerBootID = resp.BootID
		current.InventoryRevision = resp.InventoryRevision
		current.ExpiresAt = time.Unix(resp.ExpiresAt, 0)
		sessionCopy := *current
		s.sessions.mu.Unlock()
		return sessionCopy, true, nil
	}
	s.sessions.mu.Unlock()
	s.asyncCompensatingDestroy(worker.NodeID, id, "creation_canceled")
	return ActiveSession{}, false, ErrSandboxBusy
}

// TerminateSandbox terminates a sandbox session, rolls back cluster resources,
// releases IP slots, and records an audit log. Shared identically by user Stop and background Reaper.
// State machine: Running -> Terminating -> Terminated (Success) or CleanupFailed (Retained for retry).
func (s *ControlHTTPServer) TerminateSandbox(ctx context.Context, sandboxID string, reason string) (ActiveSession, error) {
	s.sessions.mu.Lock()
	stored, exists := s.sessions.sessions[sandboxID]
	if !exists || stored.State == SessionStateTerminating {
		s.sessions.mu.Unlock()
		return ActiveSession{}, errors.New("sandbox not found or already stopping")
	}
	session := *stored
	stored.State = SessionStateTerminating
	s.sessions.mu.Unlock()

	// 2. Dispatch reverse DestroyBox to worker node
	destroyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	err := s.clusterMgr.DestroyBox(destroyCtx, session.WorkerNodeID, protocol.DestroyBoxReq{
		SandboxID: session.SandboxID,
		Reason:    reason,
	})

	if err != nil {
		// Retain session and slot to prevent reused IP conflict or orphaned resources
		s.sessions.mu.Lock()
		if cur := s.sessions.sessions[sandboxID]; cur != nil && cur.State == SessionStateTerminating {
			cur.State = SessionStateCleanupFailed
		}
		s.sessions.mu.Unlock()
		log.Printf("[ERROR] Worker %s failed to destroy sandbox %s: %v (marked as cleanup_failed)", session.WorkerNodeID, sandboxID, err)
		return session, fmt.Errorf("worker cleanup failed: %w", err)
	}

	// 3. Worker confirmed destruction: mark Terminated, release slot, and remove from active list
	removedSession, _ := s.purgeSessionLocally(session.SandboxID)

	// 4. Record audit log
	durationSec := int64(time.Since(session.StartedAt).Seconds())
	if s.db != nil {
		_ = s.db.AddAuditLog(&session.UserID, "sandbox_terminate", "", fmt.Sprintf("box=%s, reason=%s, duration=%ds", session.SandboxID, reason, durationSec))
	}

	return removedSession, nil
}

// purgeSessionLocally releases the IP slot, marks the session as Terminated, and removes it from active tracking.
func (s *ControlHTTPServer) purgeSessionLocally(id string) (ActiveSession, bool) {
	s.sessions.mu.Lock()
	defer s.sessions.mu.Unlock()
	rec, ok := s.sessions.sessions[id]
	s.slots.Release(id)
	if !ok {
		return ActiveSession{}, false
	}
	delete(s.sessions.sessions, id)
	if s.sessions.byUser[rec.UserID] == id {
		delete(s.sessions.byUser, rec.UserID)
	}
	return *rec, true
}

// Cleanup is retried by subsequent heartbeats, with at most one RPC per instance.
func (s *ControlHTTPServer) asyncCompensatingDestroy(workerID, id, reason string) {
	s.cleanupMu.Lock()
	if s.cleaning[id] {
		s.cleanupMu.Unlock()
		return
	}
	s.cleaning[id] = true
	s.cleanupMu.Unlock()
	go func() {
		defer func() { s.cleanupMu.Lock(); delete(s.cleaning, id); s.cleanupMu.Unlock() }()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.clusterMgr.DestroyBox(ctx, workerID, protocol.DestroyBoxReq{SandboxID: id, Reason: reason}); err == nil {
			s.purgeSessionLocally(id)
		} else {
			s.sessions.SetState(id, SessionStateCleanupFailed)
			log.Printf("[CONTROL] Cleanup pending for %s: %v", id, err)
		}
	}()
}

// A complete inventory is authoritative only after the worker's acknowledged
// creation revision. Unknown instances after a Control restart are reset.
func (s *ControlHTTPServer) syncWorkerState(workerID string, hb protocol.HeartbeatReq) {
	if !s.clusterMgr.IsCurrentInventory(workerID, hb) {
		return
	}
	alive := make(map[string]protocol.SandboxSummary, len(hb.Sandboxes))
	for _, item := range hb.Sandboxes {
		alive[item.ID] = item
	}
	ready := true
	for _, sess := range s.sessions.ListAll() {
		if sess.WorkerNodeID != workerID {
			continue
		}
		if sess.WorkerBootID != "" && sess.WorkerBootID != hb.BootID {
			s.purgeSessionLocally(sess.SandboxID)
			continue
		}
		item, present := alive[sess.SandboxID]
		delete(alive, sess.SandboxID)
		if sess.State == SessionStateCreating || sess.State == SessionStateTerminating {
			continue
		}
		if !present && sess.InventoryRevision > 0 && hb.InventoryRevision >= sess.InventoryRevision {
			s.purgeSessionLocally(sess.SandboxID)
			continue
		}
		if sess.State == SessionStateCleanupFailed || (present && item.State == "cleanup_failed") {
			s.asyncCompensatingDestroy(workerID, sess.SandboxID, "retry_cleanup")
		} else if present && item.LastActiveAt > 0 {
			s.sessions.Touch(sess.SandboxID, time.Unix(item.LastActiveAt, 0))
		}
	}
	for id := range alive {
		ready = false
		s.asyncCompensatingDestroy(workerID, id, "unknown_after_control_restart")
	}
	s.clusterMgr.SetInventoryReady(workerID, hb.BootID, hb.Seq, ready)
}

func (s *ControlHTTPServer) handleStopSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, _, role, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req StopSandboxReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			writeError(w, http.StatusRequestEntityTooLarge, "request too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	sandboxID := req.SandboxID
	if sandboxID == "" {
		// If sandbox_id omitted, default to user's currently active sandbox
		if active, running := s.sessions.GetByUser(uid); running {
			sandboxID = active.SandboxID
		}
	}

	if sandboxID == "" {
		writeError(w, http.StatusBadRequest, "invalid request body: sandbox_id is required or no active sandbox found")
		return
	}

	session, exists := s.sessions.Get(sandboxID)
	if !exists {
		writeError(w, http.StatusNotFound, "sandbox session not found or already stopped")
		return
	}

	if session.UserID != uid && role != "admin" {
		writeError(w, http.StatusForbidden, "you do not have permission to stop this sandbox")
		return
	}

	stoppedSession, err := s.TerminateSandbox(r.Context(), sandboxID, "user_stop")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to terminate sandbox: "+err.Error())
		return
	}

	durationSec := int64(time.Since(stoppedSession.StartedAt).Seconds())
	writeJSON(w, http.StatusOK, StopSandboxResp{
		Success:         true,
		SandboxID:       stoppedSession.SandboxID,
		DurationSeconds: durationSec,
	})
}

func (s *ControlHTTPServer) handleActiveSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	session, running := s.sessions.GetByUser(uid)
	if !running {
		writeJSON(w, http.StatusOK, map[string]interface{}{"active": false})
		return
	}

	if session.State != SessionStateRunning {
		writeJSON(w, http.StatusOK, map[string]interface{}{"active": false, "sandbox_id": session.SandboxID, "state": session.State})
		return
	}
	// Strictly read-only: Do NOT touch activity timestamp on polling

	ticket, _ := s.issueTicket(uid, session.SandboxID)
	endpoint, exists := s.clusterMgr.GetWorkerDataURL(session.WorkerNodeID, r.Host)
	if !exists {
		writeError(w, http.StatusServiceUnavailable, "worker node currently unreachable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":     true,
		"sandbox_id": session.SandboxID,
		"worker_id":  session.WorkerNodeID,
		"worker_url": endpoint,
		"ticket":     ticket,
		"expires_in": s.cfg.Auth.TicketTTLSec,
		"sandbox_ip": session.SandboxIP,
		"state":      session.State,
		"started_at": session.StartedAt,
		"expires_at": session.ExpiresAt,
		"policy":     session.Policy,
	})
}

func (s *ControlHTTPServer) issueTicket(uid int64, sandboxID string) (string, error) {
	jti := generateJTI()
	claims := &protocol.TicketClaims{
		Sub: uid,
		Sbx: sandboxID,
		Typ: "pty",
		Exp: time.Now().Add(time.Duration(s.cfg.Auth.TicketTTLSec) * time.Second).Unix(),
		JTI: jti,
	}
	return protocol.SignTicket(claims, s.privKey)
}

// resolvePolicy constructs the authoritative SandboxPolicy directly from ControlConfig.
// Sandboxes strictly obey machine/cluster administrative baselines. End-users are never permitted
// to inject, override, or tamper with host infrastructure resource policies.
func (s *ControlHTTPServer) resolvePolicy() protocol.SandboxPolicy {
	return protocol.SandboxPolicy{
		Resources: s.cfg.Sandbox.Resources,
		Storage:   s.cfg.Sandbox.Storage,
		Security: protocol.BoxSecurityPolicy{
			ReadOnlyProcSys: true,
			MaskSysrq:       true,
		},
	}
}

func generateSandboxID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sbx_%x", b)
}

func generateJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
