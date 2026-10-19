package control

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
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
		Ed25519PrivateKey string `yaml:"ed25519_private_key"`
		JWTExpireHours    int    `yaml:"jwt_expire_hours"`
		TicketTTLSec      int    `yaml:"ticket_ttl_sec"`
	} `yaml:"auth"`
	Sandbox struct {
		Resources protocol.BoxResourceQuota `yaml:"resources"`
		Storage   protocol.BoxStoragePolicy `yaml:"storage"`
		Network   struct {
			SubnetCIDR         string   `yaml:"subnet_cidr"`
			GatewayIP          string   `yaml:"gateway_ip"`
			DefaultEgressMode  string   `yaml:"default_egress_mode"`
			AllowedEgressPorts []int    `yaml:"allowed_egress_ports"`
		} `yaml:"network"`
	} `yaml:"sandbox"`
}

// LoadControlConfig reads YAML configuration or falls back to production defaults.
func LoadControlConfig(path string) (*ControlConfig, error) {
	cfg := &ControlConfig{}
	cfg.Server.ListenAddr = "127.0.0.1:18080"
	cfg.Server.CORSAllowedOrigins = []string{"*"}
	cfg.Database.Path = "/var/lib/attackbox/attackbox.db"
	cfg.Cluster.ClusterSecret = "attackbox_cluster_secret_change_me_in_prod!"
	cfg.Cluster.HeartbeatTimeoutSec = 15
	cfg.Cluster.RPCTimeoutSec = 30
	cfg.Auth.JWTExpireHours = 24
	cfg.Auth.TicketTTLSec = 60

	// Sandbox unified defaults
	cfg.Sandbox.Resources.CPUWeight = 100
	cfg.Sandbox.Resources.MemoryMaxMB = 2048
	cfg.Sandbox.Resources.MemorySwapMaxMB = 0
	cfg.Sandbox.Resources.PIDsMax = 512
	cfg.Sandbox.Resources.IOWriteBytesSec = 20971520
	cfg.Sandbox.Resources.IOWriteIOPS = 500
	cfg.Sandbox.Storage.TmpfsTmpSize = "512M"
	cfg.Sandbox.Storage.TmpfsRunSize = "256M"
	cfg.Sandbox.Network.SubnetCIDR = "10.200.0.0/16"
	cfg.Sandbox.Network.GatewayIP = "10.200.0.1"
	cfg.Sandbox.Network.DefaultEgressMode = "restricted"
	cfg.Sandbox.Network.AllowedEgressPorts = []int{53, 80, 443}

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

func GenerateInviteCode(username string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

// -------------------------------------------------------------
// 3. Slot & Session Allocation (Cluster Resource Tracking)
// -------------------------------------------------------------

var ErrNoAvailableSlots = errors.New("no available sandbox IP slots (range 1-250 exhausted)")

type SlotAllocator struct {
	mu       sync.Mutex
	maxSlots int
	slots    map[int]string
}

func NewSlotAllocator(maxSlots int) *SlotAllocator {
	if maxSlots <= 0 || maxSlots > 250 {
		maxSlots = 250
	}
	return &SlotAllocator{
		maxSlots: maxSlots,
		slots:    make(map[int]string),
	}
}

func (a *SlotAllocator) Allocate(sandboxID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := 1; i <= a.maxSlots; i++ {
		if _, exists := a.slots[i]; !exists {
			a.slots[i] = sandboxID
			return i, nil
		}
	}
	return 0, fmt.Errorf("%w: %d slots in use", ErrNoAvailableSlots, len(a.slots))
}

func (a *SlotAllocator) Release(sandboxID string) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	for slot, id := range a.slots {
		if id == sandboxID {
			delete(a.slots, slot)
			return slot
		}
	}
	return 0
}

type ActiveSession struct {
	SandboxID         string                 `json:"sandbox_id"`
	UserID            int64                  `json:"user_id"`
	WorkerNodeID      string                 `json:"worker_node_id"`
	SlotIndex         int                    `json:"slot_index"`
	SandboxIP         string                 `json:"sandbox_ip"`
	TargetIP          string                 `json:"target_ip"`
	LabID             string                 `json:"lab_id"`
	StartedAt    time.Time              `json:"started_at"`
	Policy       protocol.SandboxPolicy `json:"policy"`
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*ActiveSession
	byUser   map[int64]string
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ActiveSession),
		byUser:   make(map[int64]string),
	}
}

func (sm *SessionManager) Add(s *ActiveSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[s.SandboxID] = s
	sm.byUser[s.UserID] = s.SandboxID
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
	contextKeyUserID   contextKey = "user_id"
	contextKeyUsername contextKey = "username"
	contextKeyRole     contextKey = "role"
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
}

type StartSandboxReq struct {
	LabID        string                  `json:"lab_id"`
	CustomPolicy *protocol.SandboxPolicy `json:"custom_policy,omitempty"`
}

type StartSandboxResp struct {
	SandboxID string                 `json:"sandbox_id"`
	WorkerID  string                 `json:"worker_id"`
	WorkerURL string                 `json:"worker_url"`
	Ticket    string                 `json:"ticket"`
	ExpiresIn int                    `json:"expires_in"`
	SandboxIP string                 `json:"sandbox_ip"`
	TargetIP  string                 `json:"target_ip"`
	SlotIndex int                    `json:"slot_index"`
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
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"`
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

	// Wrap in global middlewares (Logging, CORS)
	handler := s.loggingMiddleware(mux)
	handler = s.corsMiddleware(s.cfg.Server.CORSAllowedOrigins)(handler)

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
			u, err := s.db.GetUserByID(claims.Sub)
			if err != nil || u.Status != 1 || u.TokenVersion != claims.TokenVersion {
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
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
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

func (s *ControlHTTPServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !strings.HasPrefix(r.URL.Path, "/api/agent/connect") {
			log.Printf("[HTTP] %s %s (%v)", r.Method, r.URL.Path, time.Since(start))
		}
	})
}

func (s *ControlHTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	myInviteCode := GenerateInviteCode(req.Username)
	user, err := s.db.CreateUser(req.Username, hash, req.Email, myInviteCode, nil)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
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
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.db.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil || !CheckPassword(user.PasswordHash, req.Password) {
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
			TargetIP:  existing.TargetIP,
			SlotIndex: existing.SlotIndex,
			Policy:    existing.Policy,
		})
		return
	}

	var req StartSandboxReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	_, err := s.db.GetUserByID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	mergedPolicy := s.resolvePolicy(role, req.CustomPolicy)

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
		Policy:    mergedPolicy,
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

	session := &ActiveSession{
		SandboxID:    sandboxID,
		UserID:       uid,
		WorkerNodeID: worker.NodeID,
		SlotIndex:    slotIndex,
		SandboxIP:    resp.SandboxIP,
		TargetIP:     resp.TargetIP,
		LabID:        req.LabID,
		StartedAt:    time.Now(),
		Policy:       mergedPolicy,
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
		TargetIP:  resp.TargetIP,
		SlotIndex: slotIndex,
		Policy:    mergedPolicy,
	})
	_ = role
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

	_ = s.clusterMgr.DestroyBox(r.Context(), session.WorkerNodeID, protocol.DestroyBoxReq{
		SandboxID: session.SandboxID,
		Reason:    "user_stop",
	})

	s.slots.Release(session.SandboxID)
	s.sessions.Remove(session.SandboxID)

	duration := time.Since(session.StartedAt)
	durationSec := int64(duration.Seconds())

	_ = s.db.AddAuditLog(&uid, "sandbox_stop", r.RemoteAddr, fmt.Sprintf("box=%s, duration=%ds", session.SandboxID, durationSec))

	writeJSON(w, http.StatusOK, StopSandboxResp{
		Success:         true,
		SandboxID:       session.SandboxID,
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
		"target_ip":  session.TargetIP,
		"started_at": session.StartedAt,
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

func (s *ControlHTTPServer) resolvePolicy(role string, custom *protocol.SandboxPolicy) protocol.SandboxPolicy {
	// 1. Single source of truth from s.cfg.Sandbox
	policy := protocol.SandboxPolicy{
		Resources: s.cfg.Sandbox.Resources,
		Storage:   s.cfg.Sandbox.Storage,
		Network: protocol.BoxNetworkPolicy{
			EgressMode:        s.cfg.Sandbox.Network.DefaultEgressMode,
			ExtraAllowedPorts: s.cfg.Sandbox.Network.AllowedEgressPorts,
		},
		Security: protocol.BoxSecurityPolicy{
			ReadOnlyProcSys: true,
			MaskSysrq:       true,
			AllowPTrace:     true,
			ExtraEnv:        make(map[string]string),
		},
	}

	// 2. Only admin role is permitted to override baseline quotas
	if custom != nil && role == "admin" {
		cp := *custom
		if cp.Resources.CPUWeight > 0 {
			policy.Resources.CPUWeight = cp.Resources.CPUWeight
		}
		if cp.Resources.MemoryMaxMB > 0 {
			policy.Resources.MemoryMaxMB = cp.Resources.MemoryMaxMB
		}
		if cp.Resources.PIDsMax > 0 {
			policy.Resources.PIDsMax = cp.Resources.PIDsMax
		}
		if cp.Network.EgressMode != "" {
			policy.Network.EgressMode = cp.Network.EgressMode
		}
		if len(cp.Network.ExtraAllowedPorts) > 0 {
			policy.Network.ExtraAllowedPorts = cp.Network.ExtraAllowedPorts
		}
	}

	return policy
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
