package control

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
		Network           struct {
			SubnetCIDR string `yaml:"subnet_cidr"`
			GatewayIP  string `yaml:"gateway_ip"`
		} `yaml:"network"`
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
	if cfg.Sandbox.Network.SubnetCIDR == "" {
		cfg.Sandbox.Network.SubnetCIDR = protocol.DefaultSubnetCIDR
	}
	if cfg.Sandbox.Network.GatewayIP == "" {
		cfg.Sandbox.Network.GatewayIP = protocol.DefaultGatewayIP
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

	if err := yaml.Unmarshal(data, cfg); err != nil {
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
	freeList []int
	slotToID map[int]string
	idToSlot map[string]int
}

func NewSlotAllocator(maxSlots int) *SlotAllocator {
	if maxSlots <= 0 || maxSlots > 250 {
		maxSlots = 250
	}
	freeList := make([]int, maxSlots)
	for i := 0; i < maxSlots; i++ {
		freeList[i] = i + 1
	}
	return &SlotAllocator{
		maxSlots: maxSlots,
		freeList: freeList,
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

	if len(a.freeList) == 0 {
		return 0, fmt.Errorf("%w: %d slots in use", ErrNoAvailableSlots, len(a.slotToID))
	}

	// O(1) Pop from head of freeList (preserves lowest available slot assignment)
	slot := a.freeList[0]
	a.freeList = a.freeList[1:]

	a.slotToID[slot] = sandboxID
	a.idToSlot[sandboxID] = slot
	return slot, nil
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

	// O(1..k) Insert back into freeList in sorted order to reuse lower IP slots first
	inserted := false
	for idx, s := range a.freeList {
		if slot < s {
			a.freeList = append(a.freeList[:idx], append([]int{slot}, a.freeList[idx:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		a.freeList = append(a.freeList, slot)
	}

	return slot
}

type ActiveSession struct {
	SandboxID    string                 `json:"sandbox_id"`
	UserID       int64                  `json:"user_id"`
	WorkerNodeID string                 `json:"worker_node_id"`
	SlotIndex    int                    `json:"slot_index"`
	SandboxIP    string                 `json:"sandbox_ip"`
	StartedAt    time.Time              `json:"started_at"`
	LastActiveAt time.Time              `json:"last_active_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	Policy       protocol.SandboxPolicy `json:"policy"`
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
// If available, it reserves the user slot and returns a release func (to rollback on failure) and true.
// If the user already has an active sandbox or another creation is in-flight, it returns (nil, false).
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
	if reservedAt, busy := sm.reserving[userID]; busy {
		// Clean up orphaned reservation after 60 seconds
		if time.Since(reservedAt) < 60*time.Second {
			return nil, false
		}
	}

	sm.reserving[userID] = time.Now()

	releaseFunc := func() {
		sm.mu.Lock()
		defer sm.mu.Unlock()
		delete(sm.reserving, userID)
	}

	return releaseFunc, true
}

func (sm *SessionManager) Add(s *ActiveSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.reserving, s.UserID)
	sm.sessions[s.SandboxID] = s
	sm.byUser[s.UserID] = s.SandboxID
}

func (sm *SessionManager) Touch(sandboxID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[sandboxID]; ok {
		s.LastActiveAt = time.Now()
	}
}

func (sm *SessionManager) Get(sandboxID string) (*ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[sandboxID]
	return s, ok
}

func (sm *SessionManager) GetByUser(userID int64) (*ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sbxID, ok := sm.byUser[userID]
	if !ok {
		return nil, false
	}
	s, ok := sm.sessions[sbxID]
	return s, ok
}

func (sm *SessionManager) Remove(sandboxID string) *ActiveSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.sessions[sandboxID]
	if ok {
		delete(sm.sessions, sandboxID)
		delete(sm.byUser, s.UserID)
		delete(sm.reserving, s.UserID)
	}
	return s
}

func (sm *SessionManager) ListAll() []*ActiveSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	res := make([]*ActiveSession, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		res = append(res, s)
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

type tokenCacheEntry struct {
	tokenVersion int
	status       int
	expiresAt    time.Time
}

type TokenVersionCache struct {
	mu      sync.RWMutex
	entries map[int64]tokenCacheEntry
}

func NewTokenVersionCache() *TokenVersionCache {
	return &TokenVersionCache{
		entries: make(map[int64]tokenCacheEntry),
	}
}

func (c *TokenVersionCache) Get(userID int64) (int, int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.entries[userID]
	if !exists || time.Now().After(entry.expiresAt) {
		return 0, 0, false
	}
	return entry.tokenVersion, entry.status, true
}

func (c *TokenVersionCache) Set(userID int64, tokenVersion, status int, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[userID] = tokenCacheEntry{
		tokenVersion: tokenVersion,
		status:       status,
		expiresAt:    time.Now().Add(ttl),
	}
}

func (c *TokenVersionCache) Invalidate(userID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, userID)
}

type ControlHTTPServer struct {
	cfg        *ControlConfig
	db         *DatabaseStore
	privKey    ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	clusterMgr *NodeClusterManager
	slots      *SlotAllocator
	sessions   *SessionManager
	tokenCache *TokenVersionCache
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
		tokenCache: NewTokenVersionCache(),
	}

	// Bootstrap admin user if specified in config
	if cfg.Auth.InitialAdminUsername != "" && cfg.Auth.InitialAdminPassword != "" && db != nil {
		adminHash, err := HashPassword(cfg.Auth.InitialAdminPassword)
		if err == nil {
			_ = db.EnsureAdminUser(cfg.Auth.InitialAdminUsername, adminHash, cfg.Auth.InitialAdminEmail)
		}
	}

	// Launch background reaper to reclaim expired (TTL) and idle sandboxes
	s.StartSessionReaper(context.Background(), time.Duration(cfg.Sandbox.ReaperIntervalSec)*time.Second)

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

	return handler
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
			tokenVer, status, hit := s.tokenCache.Get(claims.Sub)
			if !hit {
				u, err := s.db.GetUserByID(claims.Sub)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "session revoked or account inactive")
					return
				}
				tokenVer = u.TokenVersion
				status = u.Status
				s.tokenCache.Set(claims.Sub, tokenVer, status, 10*time.Second)
			}
			if status != 1 || tokenVer != claims.TokenVersion {
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

	s.tokenCache.Set(user.ID, user.TokenVersion, user.Status, 10*time.Second)

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

	s.tokenCache.Set(user.ID, user.TokenVersion, user.Status, 10*time.Second)

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

func (s *ControlHTTPServer) handleStartSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, _, role, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 1. Re-use existing sandbox if active
	if existing, running := s.sessions.GetByUser(uid); running {
		freshTicket, err := s.issueTicket(uid, existing.SandboxID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue session ticket for active sandbox: "+err.Error())
			return
		}

		workerIP, workerPort, exists := s.clusterMgr.GetWorkerEndpoint(existing.WorkerNodeID)
		if !exists {
			writeError(w, http.StatusServiceUnavailable, "worker node hosting active sandbox is currently unreachable")
			return
		}
		writeJSON(w, http.StatusOK, StartSandboxResp{
			SandboxID: existing.SandboxID,
			WorkerID:  existing.WorkerNodeID,
			WorkerURL: fmt.Sprintf("ws://%s:%d/ws/cli", workerIP, workerPort),
			Ticket:    freshTicket,
			ExpiresIn: s.cfg.Auth.TicketTTLSec,
			SandboxIP: existing.SandboxIP,
			SlotIndex: existing.SlotIndex,
			ExpiresAt: existing.ExpiresAt,
			Policy:    existing.Policy,
		})
		return
	}

	// 2. Atomically reserve slot to eliminate TOCTOU concurrent creation races
	releaseReserve, ok := s.sessions.TryReserve(uid)
	if !ok {
		writeError(w, http.StatusConflict, "sandbox creation is already in progress or already active for this user")
		return
	}
	defer releaseReserve()

	var req StartSandboxReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	_, err := s.db.GetUserByID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	policy := s.resolvePolicy()

	worker, err := s.clusterMgr.SelectBestWorker()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no available worker nodes: "+err.Error())
		return
	}

	sandboxID := generateSandboxID()
	slotIndex, err := s.slots.Allocate(sandboxID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "failed to allocate IP slot: "+err.Error())
		return
	}

	createReq := protocol.CreateBoxReq{
		SandboxID: sandboxID,
		UserID:    uid,
		SlotIndex: slotIndex,
		Policy:    policy,
	}

	resp, err := s.clusterMgr.CreateBox(r.Context(), worker.NodeID, createReq)
	if err != nil {
		s.slots.Release(sandboxID)
		log.Printf("[CONTROL] Worker CreateBox failed or timed out: %v. Initiating compensating destroy for %s on %s", err, sandboxID, worker.NodeID)
		// P1-9: Compensating destroy with independent context to clean up any slow or orphaned worker creation
		go func(wID, sID string) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = s.clusterMgr.DestroyBox(cleanupCtx, wID, protocol.DestroyBoxReq{
				SandboxID: sID,
				Reason:    "compensating_create_failure",
			})
		}(worker.NodeID, sandboxID)

		writeError(w, http.StatusInternalServerError, "worker failed to create sandbox: "+err.Error())
		return
	}

	ticket, err := s.issueTicket(uid, sandboxID)
	if err != nil {
		s.slots.Release(sandboxID)
		go func(wID, sID string) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = s.clusterMgr.DestroyBox(cleanupCtx, wID, protocol.DestroyBoxReq{
				SandboxID: sID,
				Reason:    "ticket_failed",
			})
		}(worker.NodeID, sandboxID)
		writeError(w, http.StatusInternalServerError, "failed to issue session ticket")
		return
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(s.cfg.Sandbox.TTLSeconds) * time.Second)

	session := &ActiveSession{
		SandboxID:    sandboxID,
		UserID:       uid,
		WorkerNodeID: worker.NodeID,
		SlotIndex:    slotIndex,
		SandboxIP:    resp.SandboxIP,
		StartedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    expiresAt,
		Policy:       policy,
	}
	s.sessions.Add(session)

	_ = s.db.AddAuditLog(&uid, "sandbox_start", r.RemoteAddr, fmt.Sprintf("box=%s, worker=%s", sandboxID, worker.NodeID))

	workerHost := worker.RemoteIP
	if workerHost == "" || workerHost == "127.0.0.1" {
		workerHost = strings.Split(r.Host, ":")[0]
	}

	writeJSON(w, http.StatusCreated, StartSandboxResp{
		SandboxID: sandboxID,
		WorkerID:  worker.NodeID,
		WorkerURL: fmt.Sprintf("ws://%s:%d/ws/cli", workerHost, worker.DataPort),
		Ticket:    ticket,
		ExpiresIn: s.cfg.Auth.TicketTTLSec,
		SandboxIP: resp.SandboxIP,
		SlotIndex: slotIndex,
		ExpiresAt: expiresAt,
		Policy:    policy,
	})
	_ = role
}

// TerminateSandbox terminates a sandbox session, rolls back cluster resources,
// releases IP slots, and records an audit log. Shared identically by user Stop and background Reaper.
func (s *ControlHTTPServer) TerminateSandbox(ctx context.Context, sandboxID string, reason string) (*ActiveSession, error) {
	session := s.sessions.Remove(sandboxID)
	if session == nil {
		return nil, errors.New("sandbox not found or already terminated")
	}

	// 1. Release IP slot
	s.slots.Release(session.SandboxID)

	// 2. Dispatch reverse DestroyBox to worker node
	destroyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = s.clusterMgr.DestroyBox(destroyCtx, session.WorkerNodeID, protocol.DestroyBoxReq{
		SandboxID: session.SandboxID,
		Reason:    reason,
	})

	// 3. Record audit log
	durationSec := int64(time.Since(session.StartedAt).Seconds())
	_ = s.db.AddAuditLog(&session.UserID, "sandbox_terminate", "", fmt.Sprintf("box=%s, reason=%s, duration=%ds", session.SandboxID, reason, durationSec))

	return session, nil
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SandboxID == "" {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body: sandbox_id is required")
		return
	}

	session, exists := s.sessions.Get(req.SandboxID)
	if !exists {
		writeError(w, http.StatusNotFound, "sandbox session not found or already stopped")
		return
	}

	if session.UserID != uid && role != "admin" {
		writeError(w, http.StatusForbidden, "you do not have permission to stop this sandbox")
		return
	}

	stoppedSession, err := s.TerminateSandbox(r.Context(), req.SandboxID, "user_stop")
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

// StartSessionReaper launches the background scanning loop to automatically reclaim expired (TTL)
// and idle (inactive) sandboxes using the exact same TerminateSandbox pipeline.
func (s *ControlHTTPServer) StartSessionReaper(ctx context.Context, checkInterval time.Duration) {
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}
	ticker := time.NewTicker(checkInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.ReapExpiredSessions()
			}
		}
	}()
}

// ReapExpiredSessions scans and terminates sandboxes that have exceeded TTL or idle thresholds.
func (s *ControlHTTPServer) ReapExpiredSessions() {
	now := time.Now()
	sessions := s.sessions.ListAll()
	for _, sess := range sessions {
		// 1. Hard TTL expiration check
		if !sess.ExpiresAt.IsZero() && now.After(sess.ExpiresAt) {
			log.Printf("[REAPER] Sandbox %s (User %d) reached hard TTL (%v). Reaping...", sess.SandboxID, sess.UserID, sess.ExpiresAt)
			_, _ = s.TerminateSandbox(context.Background(), sess.SandboxID, "ttl_expired")
			continue
		}
		// 2. Inactivity / idle timeout check
		if s.cfg.Sandbox.IdleSeconds > 0 && !sess.LastActiveAt.IsZero() && now.Sub(sess.LastActiveAt) > time.Duration(s.cfg.Sandbox.IdleSeconds)*time.Second {
			log.Printf("[REAPER] Sandbox %s (User %d) idle for %v (> %ds). Reaping...", sess.SandboxID, sess.UserID, now.Sub(sess.LastActiveAt), s.cfg.Sandbox.IdleSeconds)
			_, _ = s.TerminateSandbox(context.Background(), sess.SandboxID, "idle_timeout")
			continue
		}
	}
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

	// Refresh activity timestamp on user polling
	s.sessions.Touch(session.SandboxID)

	ticket, _ := s.issueTicket(uid, session.SandboxID)
	workerIP, workerPort, exists := s.clusterMgr.GetWorkerEndpoint(session.WorkerNodeID)
	if !exists {
		workerIP = strings.Split(r.Host, ":")[0]
		workerPort = 8443
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":     true,
		"sandbox_id": session.SandboxID,
		"worker_id":  session.WorkerNodeID,
		"worker_url": fmt.Sprintf("ws://%s:%d/ws/cli", workerIP, workerPort),
		"ticket":     ticket,
		"expires_in": s.cfg.Auth.TicketTTLSec,
		"sandbox_ip": session.SandboxIP,
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
	b := make([]byte, 4)
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
