package node

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"attackbox/node/runtime"
	"attackbox/protocol"
)

// GatewayConfig represents external interactive gateway listener parameters.
type GatewayConfig struct {
	ListenAddr       string `yaml:"listen_addr"`
	TLSEnabled       bool   `yaml:"tls_enabled"`
	TLSCert          string `yaml:"tls_cert"`
	TLSKey           string `yaml:"tls_key"`
	Ed25519PublicKey string `yaml:"ed25519_public_key"`
}

// -------------------------------------------------------------
// Interactive Terminal PTY Session Management
// -------------------------------------------------------------

type PTYSession struct {
	Master *os.File
	Cmd    *exec.Cmd
	mu     sync.Mutex
	closed bool
}

func StartPTY(targetPID int, cgroupPath string, rows, cols uint16) (*PTYSession, error) {
	if rows == 0 {
		rows = 35
	}
	if cols == 0 {
		cols = 110
	}

	// 1. Locate nsenter binary (check PATH and fallback standard paths, avoiding hardcoded single location)
	nsenterBin, err := exec.LookPath("nsenter")
	if err != nil {
		for _, fallback := range []string{"/usr/bin/nsenter", "/bin/nsenter", "/usr/local/bin/nsenter"} {
			if _, statErr := os.Stat(fallback); statErr == nil {
				nsenterBin = fallback
				break
			}
		}
	}
	if nsenterBin == "" {
		return nil, fmt.Errorf("nsenter binary not found in PATH or standard system locations")
	}

	wrapperPath := fmt.Sprintf("/proc/%d/root/usr/bin/attackbox-enter", targetPID)
	var cmdArgs []string
	if _, err := os.Stat(wrapperPath); err == nil {
		cmdArgs = []string{
			"--target", strconv.Itoa(targetPID),
			"--mount", "--uts", "--ipc", "--net", "--pid",
			"--root", "--wd",
			"/usr/bin/attackbox-enter", "/bin/bash", "-l",
		}
	} else {
		cmdArgs = []string{
			"--target", strconv.Itoa(targetPID),
			"--mount", "--uts", "--ipc", "--net", "--pid",
			"--root", "--wd",
			"/bin/bash", "-l",
		}
	}

	cmd := exec.Command(nsenterBin, cmdArgs...)
	cmd.Env = []string{
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/home/box",
		"USER=box",
		"LOGNAME=box",
		"SHELL=/bin/bash",
	}

	// 2. Open cgroup file descriptor for native Linux 5.7+ CLONE_INTO_CGROUP
	// Ensures child process is born directly inside the sandbox cgroup, eliminating race window.
	var cgFile *os.File
	if cgroupPath != "" {
		var openErr error
		cgFile, openErr = os.Open(cgroupPath)
		if openErr != nil {
			return nil, fmt.Errorf("open sandbox cgroup %s failed: %w", cgroupPath, openErr)
		}
		defer cgFile.Close()

		cmd.SysProcAttr = &syscall.SysProcAttr{
			UseCgroupFD: true,
			CgroupFD:    int(cgFile.Fd()),
		}
	}

	// 3. Start PTY session with atomic cgroup assignment
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, fmt.Errorf("pty.StartWithSize: %w", err)
	}

	// 4. Secondary cgroup.procs sync with strict error checking (Fail-Closed)
	if cgroupPath != "" && cmd.Process != nil {
		cgProcs := filepath.Join(cgroupPath, "cgroup.procs")
		if writeErr := os.WriteFile(cgProcs, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); writeErr != nil {
			// Fail-closed: kill process immediately, close ptmx, and return error
			_ = cmd.Process.Kill()
			_ = ptmx.Close()
			_ = cmd.Wait()
			return nil, fmt.Errorf("failed to bind PTY process %d to cgroup %s: %w", cmd.Process.Pid, cgroupPath, writeErr)
		}
	}

	return &PTYSession{
		Master: ptmx,
		Cmd:    cmd,
	}, nil
}

func (s *PTYSession) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.Master == nil {
		return nil
	}
	return pty.Setsize(s.Master, &pty.Winsize{Rows: rows, Cols: cols})
}

func (s *PTYSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	if s.Cmd != nil && s.Cmd.Process != nil {
		_ = s.Cmd.Process.Kill()
	}
	if s.Master != nil {
		_ = s.Master.Close()
	}
	if s.Cmd != nil {
		_ = s.Cmd.Wait()
	}
	return nil
}

type SafeConn struct {
	*websocket.Conn
	writeMu sync.Mutex
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{Conn: conn}
}

func (c *SafeConn) WriteMessage(messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

func (c *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.Conn.WriteControl(messageType, data, deadline)
}

type GatewaySessionManager struct {
	mu          sync.Mutex
	cliSessions map[string]*SafeConn
}

func NewGatewaySessionManager() *GatewaySessionManager {
	return &GatewaySessionManager{
		cliSessions: make(map[string]*SafeConn),
	}
}

func (sm *GatewaySessionManager) RegisterCLI(key string, conn *SafeConn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if oldConn, exists := sm.cliSessions[key]; exists {
		_ = oldConn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "preempted by new connection"),
			time.Now().Add(time.Second),
		)
		_ = oldConn.Close()
	}
	sm.cliSessions[key] = conn
}

func (sm *GatewaySessionManager) UnregisterCLI(key string, conn *SafeConn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cur, exists := sm.cliSessions[key]; exists && cur == conn {
		delete(sm.cliSessions, key)
	}
}

// -------------------------------------------------------------
// Container Gateway Server (:8443)
// -------------------------------------------------------------

// ContainerGatewayServer handles direct client data connections (:8443) for CLI PTY and GUI streams.
type ContainerGatewayServer struct {
	cfg          GatewayConfig
	rtMgr        *runtime.RuntimeManager
	sessions     *GatewaySessionManager
	replayFilter *protocol.JTIReplayFilter
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
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				return strings.EqualFold(u.Host, r.Host)
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
	s.pubKey = pubKey
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

	if err := s.replayFilter.CheckAndConsume(claims.JTI, 60*time.Second); err != nil {
		http.Error(w, "ticket jti already consumed (replay)", http.StatusForbidden)
		return
	}

	if claims.Typ != "pty" {
		http.Error(w, "invalid stream type for CLI endpoint", http.StatusBadRequest)
		return
	}

	sbx, ok := s.rtMgr.GetSandbox(claims.Sbx)
	if !ok || sbx.PID <= 0 {
		http.Error(w, "target sandbox is not active", http.StatusNotFound)
		return
	}

	rawConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[DATA] WebSocket upgrade error: %v", err)
		return
	}
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

	ptySess, err := StartPTY(sbx.PID, sbx.CgroupPath, 35, 110)
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
			_ = c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
		}
	}
}
