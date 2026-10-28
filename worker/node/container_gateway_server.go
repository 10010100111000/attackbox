package node

import (
	"attackbox/node/runtime"
	"attackbox/protocol"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ContainerGatewayServer handles direct client data connections (:8443) for CLI PTY and GUI streams.
type ContainerGatewayServer struct {
	cfg          GatewayConfig
	rtMgr        *runtime.RuntimeManager
	sessions     *GatewaySessionManager
	replayFilter *protocol.JTIReplayFilter
	keyMu        sync.RWMutex
	pubKey       ed25519.PublicKey
	httpServer   *http.Server
	upgrader     websocket.Upgrader
}

// NewContainerGatewayServer initializes the external interactive stream gateway.
func NewContainerGatewayServer(cfg GatewayConfig, rtMgr *runtime.RuntimeManager) (*ContainerGatewayServer, error) {
	s := &ContainerGatewayServer{
		cfg:          cfg,
		rtMgr:        rtMgr,
		sessions:     NewGatewaySessionManager(),
		replayFilter: protocol.NewJTIReplayFilter(1 * time.Minute),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1 << 15,
			WriteBufferSize: 1 << 15,
			Subprotocols:    []string{"binary"},
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				if len(cfg.AllowedOrigins) == 0 {
					return true
				}
				for _, allowed := range cfg.AllowedOrigins {
					if allowed == "*" {
						return true
					}
				}
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				if strings.EqualFold(u.Host, r.Host) {
					return true
				}
				for _, allowed := range cfg.AllowedOrigins {
					if strings.EqualFold(u.Host, allowed) || strings.EqualFold(origin, allowed) {
						return true
					}
				}
				return false
			},
		},
	}

	if cfg.Ed25519PublicKey != "" {
		keyBytes, err := hex.DecodeString(strings.TrimSpace(cfg.Ed25519PublicKey))
		if err != nil || len(keyBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid ed25519 public key: %w", err)
		}
		s.pubKey = ed25519.PublicKey(keyBytes)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws/cli", s.handleCLI)
	mux.HandleFunc("/ws/gui", s.handleGUI)

	listenAddr := cfg.ListenAddr
	if listenAddr == "" {
		listenAddr = ":8443"
	}

	s.httpServer = &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s, nil
}

func (s *ContainerGatewayServer) SetPublicKey(pubKey ed25519.PublicKey) {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	s.pubKey = pubKey
}

func (s *ContainerGatewayServer) getPublicKey() ed25519.PublicKey {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	return s.pubKey
}

func (s *ContainerGatewayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpServer.Handler.ServeHTTP(w, r)
}

func (s *ContainerGatewayServer) Start() error {
	if s.cfg.TLSEnabled {
		return s.httpServer.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
	}
	return s.httpServer.ListenAndServe()
}

func (s *ContainerGatewayServer) Stop(ctx context.Context) error {
	s.replayFilter.Close()
	return s.httpServer.Shutdown(ctx)
}

func (s *ContainerGatewayServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK\n"))
}

func (s *ContainerGatewayServer) handleCLI(w http.ResponseWriter, r *http.Request) {
	rawTicket := r.URL.Query().Get("ticket")
	if rawTicket == "" {
		http.Error(w, "missing ticket query parameter", http.StatusUnauthorized)
		return
	}

	pubKey := s.getPublicKey()
	if len(pubKey) == 0 {
		http.Error(w, "server ed25519 key not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := protocol.VerifyTicket(rawTicket, pubKey)
	if err != nil {
		log.Printf("[DATA] Ticket verification rejected: %v", err)
		http.Error(w, fmt.Sprintf("invalid ticket: %v", err), http.StatusForbidden)
		return
	}

	if err := s.replayFilter.CheckAndConsume(claims.JTI, 60*time.Second); err != nil {
		http.Error(w, "ticket jti already consumed (replay)", http.StatusForbidden)
		return
	}

	if claims.Typ != "pty" {
		http.Error(w, "invalid stream type for CLI endpoint", http.StatusBadRequest)
		return
	}

	sbx, ok := s.rtMgr.GetSnapshot(claims.Sbx)
	if !ok || sbx.PID <= 0 || sbx.State != "running" {
		http.Error(w, "target sandbox is not active", http.StatusNotFound)
		return
	}

	rawConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[DATA] WebSocket upgrade error: %v", err)
		return
	}
	// Enforce strict upper bound on inbound PTY chunks to prevent memory exhaustion
	rawConn.SetReadLimit(protocol.MaxPTYFrameSize)
	conn := NewSafeConn(rawConn)
	defer conn.Close()

	winID := r.URL.Query().Get("win")
	if winID == "" {
		winID = "default"
	}
	sessKey := claims.Sbx + ":" + winID

	s.sessions.RegisterCLI(sessKey, conn)
	defer s.sessions.UnregisterCLI(sessKey, conn)

	stopKeepalive := make(chan struct{})
	go s.keepalive(conn, stopKeepalive)
	defer close(stopKeepalive)

	cgPath := s.rtMgr.CgroupPath(claims.Sbx)
	ptySess, err := StartPTY(sbx.PID, cgPath, 35, 110)
	if err != nil {
		log.Printf("[DATA] Failed to start PTY for %s: %v", claims.Sbx, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("cannot spawn terminal: "+err.Error()))
		return
	}
	defer ptySess.Close()

	done := make(chan struct{}, 2)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptySess.Master.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	go func() {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if mt == websocket.TextMessage {
				var sz struct {
					Cols uint16 `json:"cols"`
					Rows uint16 `json:"rows"`
				}
				if json.Unmarshal(msg, &sz) == nil && sz.Cols > 0 && sz.Rows > 0 {
					_ = ptySess.Resize(sz.Rows, sz.Cols)
					continue
				}
			}
			if len(msg) > 0 {
				s.rtMgr.TouchSandbox(claims.Sbx)
			}
			if _, err := ptySess.Master.Write(msg); err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
}

func (s *ContainerGatewayServer) keepalive(c *SafeConn, stop <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				_ = c.Close()
				return
			}
		}
	}
}

func (s *ContainerGatewayServer) handleGUI(w http.ResponseWriter, r *http.Request) {
	rawTicket := r.URL.Query().Get("ticket")
	if rawTicket == "" {
		http.Error(w, "missing ticket query parameter", http.StatusUnauthorized)
		return
	}

	pubKey := s.getPublicKey()
	if len(pubKey) == 0 {
		http.Error(w, "server ed25519 key not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := protocol.VerifyTicket(rawTicket, pubKey)
	if err != nil {
		log.Printf("[DATA] GUI Ticket verification rejected: %v", err)
		http.Error(w, fmt.Sprintf("invalid ticket: %v", err), http.StatusForbidden)
		return
	}

	if err := s.replayFilter.CheckAndConsume(claims.JTI, 60*time.Second); err != nil {
		http.Error(w, "ticket jti already consumed (replay)", http.StatusForbidden)
		return
	}

	if claims.Typ != "vnc" {
		http.Error(w, "invalid stream type for GUI endpoint", http.StatusBadRequest)
		return
	}

	sbx, ok := s.rtMgr.GetSnapshot(claims.Sbx)
	if !ok || sbx.PID <= 0 || sbx.State != "running" {
		http.Error(w, "target sandbox is not active", http.StatusNotFound)
		return
	}

	// 1. 按需确保沙箱内部的 GUI 桌面 (TigerVNC + MATE) 已就绪
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	if err := s.rtMgr.EnsureGUI(ctx, sbx.ID); err != nil {
		log.Printf("[DATA] EnsureGUI failed for %s: %v", sbx.ID, err)
		http.Error(w, "failed to start desktop environment", http.StatusInternalServerError)
		return
	}

	// 2. 取得宿主机可访问的沙箱 gui.sock 路径
	sockPath, err := s.rtMgr.GetGUISocketPath(sbx.ID)
	if err != nil {
		http.Error(w, "cannot locate sandbox gui socket", http.StatusInternalServerError)
		return
	}

	// 3. 升级 WebSocket
	rawConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[DATA] WebSocket upgrade error for GUI %s: %v", claims.Sbx, err)
		return
	}
	// 4. 注册 GUI 会话并执行单连接互斥抢占
	rawConn.SetReadLimit(protocol.MaxWebSocketMessageSize)
	conn := NewSafeConn(rawConn)
	defer conn.Close()
	s.sessions.RegisterGUI(sbx.ID, conn)
	defer s.sessions.UnregisterGUI(sbx.ID, conn)

	// 5. 连接沙箱内部的 Unix Socket
	unixConn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		log.Printf("[DATA] Dial gui.sock %s failed: %v", sockPath, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("box desktop socket unavailable"))
		return
	}
	defer unixConn.Close()

	stopKeepalive := make(chan struct{})
	go s.keepalive(conn, stopKeepalive)
	defer close(stopKeepalive)

	done := make(chan struct{}, 2)

	// WebSocket -> Unix Domain Socket
	go func() {
		defer func() { done <- struct{}{} }()
		activity := &rfbActivity{}
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.BinaryMessage && mt != websocket.TextMessage {
				continue
			}
			if activity.Observe(msg) {
				s.rtMgr.TouchSandbox(claims.Sbx)
			}
			if _, err := unixConn.Write(msg); err != nil {
				return
			}
		}
	}()

	// Unix Domain Socket -> WebSocket
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 1<<15)
		for {
			n, err := unixConn.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
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
