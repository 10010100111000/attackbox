package dataserver

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/internal/worker/config"
	"attackbox/internal/worker/interactive"
	"attackbox/internal/worker/runtime"
	"attackbox/pkg/protocol"
)

// Server handles direct client data connections (:8443) for PTY and GUI streams.
type Server struct {
	cfg          config.DataServerConfig
	rtMgr        *runtime.RuntimeManager
	sessions     *interactive.SessionManager
	replayFilter *protocol.JTIReplayFilter
	pubKey       ed25519.PublicKey
	httpServer   *http.Server
	upgrader     websocket.Upgrader
}

// NewServer initializes the external data channel server.
func NewServer(cfg config.DataServerConfig, rtMgr *runtime.RuntimeManager) (*Server, error) {
	s := &Server{
		cfg:          cfg,
		rtMgr:        rtMgr,
		sessions:     interactive.NewSessionManager(),
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
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				return strings.EqualFold(u.Host, r.Host)
			},
		},
	}

	// Parse Ed25519 Public Key if provided
	if cfg.Ed25519PublicKey != "" {
		keyBytes, err := hex.DecodeString(strings.TrimSpace(cfg.Ed25519PublicKey))
		if err != nil || len(keyBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid ed25519 public key (must be 32-byte hex): %w", err)
		}
		s.pubKey = ed25519.PublicKey(keyBytes)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws/cli", s.handleCLI)

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

// SetPublicKey dynamically updates the Ed25519 public key (e.g. synced from Control during handshake).
func (s *Server) SetPublicKey(pubKey ed25519.PublicKey) {
	s.pubKey = pubKey
}

// Start begins listening on the configured data port.
func (s *Server) Start() error {
	if s.cfg.TLSEnabled {
		return s.httpServer.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
	}
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the data server.
func (s *Server) Stop(ctx context.Context) error {
	s.replayFilter.Close()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK\n"))
}

func (s *Server) handleCLI(w http.ResponseWriter, r *http.Request) {
	rawTicket := r.URL.Query().Get("ticket")
	if rawTicket == "" {
		http.Error(w, "missing ticket query parameter", http.StatusUnauthorized)
		return
	}

	// 1. Pure in-memory offline Ed25519 Ticket verification
	if len(s.pubKey) == 0 {
		http.Error(w, "server ed25519 key not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := protocol.VerifyTicket(rawTicket, s.pubKey)
	if err != nil {
		log.Printf("[DATA] Ticket verification rejected: %v", err)
		http.Error(w, fmt.Sprintf("invalid ticket: %v", err), http.StatusForbidden)
		return
	}

	// JTI Replay Prevention
	if err := s.replayFilter.CheckAndConsume(claims.JTI, 60*time.Second); err != nil {
		http.Error(w, "ticket jti already consumed (replay)", http.StatusForbidden)
		return
	}

	if claims.Typ != "pty" {
		http.Error(w, "invalid stream type for CLI endpoint", http.StatusBadRequest)
		return
	}

	// 2. Locate target sandbox
	sbx, ok := s.rtMgr.GetSandbox(claims.Sbx)
	if !ok || sbx.PID <= 0 {
		http.Error(w, "target sandbox is not active", http.StatusNotFound)
		return
	}

	// 3. Upgrade to WebSocket
	rawConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[DATA] WebSocket upgrade error: %v", err)
		return
	}
	conn := interactive.NewSafeConn(rawConn)
	defer conn.Close()

	winID := r.URL.Query().Get("win")
	if winID == "" {
		winID = "default"
	}
	sessKey := claims.Sbx + ":" + winID

	// 4. Enforce single-active session preemption
	s.sessions.RegisterCLI(sessKey, conn)
	defer s.sessions.UnregisterCLI(sessKey, conn)

	// 5. Cloudflare idle keepalive (25s ping)
	stopKeepalive := make(chan struct{})
	go s.keepalive(conn, stopKeepalive)
	defer close(stopKeepalive)

	// 6. Spawn PTY into sandbox namespaces via nsenter
	ptySess, err := interactive.StartPTY(sbx.PID, sbx.CgroupPath, 35, 110)
	if err != nil {
		log.Printf("[DATA] Failed to start PTY for %s: %v", claims.Sbx, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("cannot spawn terminal: "+err.Error()))
		return
	}
	defer ptySess.Close()

	done := make(chan struct{}, 2)

	// Output goroutine: PTY Master -> WebSocket BinaryMessage
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

	// Input loop: WebSocket -> PTY Master (with JSON resize handling)
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
			if _, err := ptySess.Master.Write(msg); err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	// Wait until either direction closes
	<-done
}

func (s *Server) keepalive(c *interactive.SafeConn, stop <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			_ = c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
		}
	}
}
