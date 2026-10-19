package agent

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/internal/worker/config"
	"attackbox/internal/worker/network"
	rt "attackbox/internal/worker/runtime"
	"attackbox/pkg/protocol"
)

// Agent manages the reverse WebSocket control channel from Worker to Control Hub.
type Agent struct {
	cfg       *config.Config
	rtMgr     *rt.RuntimeManager
	conn      *websocket.Conn
	connMu    sync.Mutex
	slotsMu   sync.Mutex
	slots     map[string]int
	stopCh    chan struct{}
	wg        sync.WaitGroup
	onKeySync func(pubKey ed25519.PublicKey)
}

// NewAgent constructs a new Worker Agent client.
func NewAgent(cfg *config.Config, rtMgr *rt.RuntimeManager, onKeySync func(pubKey ed25519.PublicKey)) *Agent {
	return &Agent{
		cfg:       cfg,
		rtMgr:     rtMgr,
		slots:     make(map[string]int),
		stopCh:    make(chan struct{}),
		onKeySync: onKeySync,
	}
}

// Start initiates the reverse connection loop with automatic reconnect.
func (a *Agent) Start(ctx context.Context) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.connectLoop(ctx)
	}()
}

// Stop terminates the reverse agent connection.
func (a *Agent) Stop() {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}

	a.connMu.Lock()
	if a.conn != nil {
		_ = a.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "worker shutting down"),
			time.Now().Add(time.Second),
		)
		_ = a.conn.Close()
	}
	a.connMu.Unlock()

	a.wg.Wait()
}

func (a *Agent) connectLoop(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 15 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}

		err := a.runSession(ctx)
		if err != nil {
			log.Printf("[AGENT] Control channel disconnected: %v (retrying in %v)", err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (a *Agent) runSession(ctx context.Context) error {
	controlURL := a.cfg.Node.ControlURL
	if controlURL == "" {
		return fmt.Errorf("control_url is empty")
	}

	u, err := url.Parse(controlURL)
	if err != nil {
		return fmt.Errorf("invalid control_url: %w", err)
	}

	headers := http.Header{}
	if a.cfg.Node.ClusterSecret != "" {
		headers.Set("Authorization", "Bearer "+a.cfg.Node.ClusterSecret)
	}
	headers.Set("X-Node-ID", a.cfg.Node.NodeID)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("dial control hub: %w", err)
	}

	a.connMu.Lock()
	a.conn = conn
	a.connMu.Unlock()
	defer func() {
		a.connMu.Lock()
		if a.conn == conn {
			_ = conn.Close()
			a.conn = nil
		}
		a.connMu.Unlock()
	}()

	log.Printf("[AGENT] Connected to Control Hub at %s", controlURL)

	// 1. Send Registration frame
	regPayload := protocol.RegisterReq{
		NodeID:   a.cfg.Node.NodeID,
		Version:  "2.0",
		DataPort: 8443,
		Capabilities: protocol.NodeCapabilities{
			HasOverlayFS: true,
			HasKVM:       hasKVMSupport(),
			HasDocker:    true,
			CPUCores:     runtime.NumCPU(),
			TotalMemMB:   65536,
			OSVersion:    runtime.GOOS + "/" + runtime.GOARCH,
		},
	}

	regMsg, err := protocol.NewMessage(protocol.MsgTypeRegister, regPayload)
	if err != nil {
		return fmt.Errorf("create register message: %w", err)
	}
	if err := a.sendMessage(regMsg); err != nil {
		return fmt.Errorf("send register message: %w", err)
	}

	// 2. Start periodic heartbeat goroutine
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	go a.heartbeatLoop(sessionCtx)

	// 3. Receive & Dispatch loop
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read control message: %w", err)
		}

		var msg protocol.Message
		if err := protocol.UnpackMessage(rawMsg, &msg); err != nil {
			log.Printf("[AGENT] Failed to parse message frame: %v", err)
			continue
		}

		go a.dispatchMessage(&msg)
	}
}

func (a *Agent) sendMessage(msg *protocol.Message) error {
	data, err := protocol.PackMessage(msg)
	if err != nil {
		return fmt.Errorf("pack message: %w", err)
	}

	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn == nil {
		return fmt.Errorf("connection closed")
	}
	return a.conn.WriteMessage(websocket.TextMessage, data)
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	interval := time.Duration(a.cfg.Node.HeartbeatIntervalSec) * time.Second
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			hbPayload := protocol.HeartbeatReq{
				NodeID:          a.cfg.Node.NodeID,
				Timestamp:       time.Now().Unix(),
				CPUUsagePercent: 5.0,
				MemFreeMB:       32768,
				DiskFreeGB:      a.rtMgr.DiskGuard().GetCurrentFreeMB() / 1024,
				ActiveSandboxes: len(a.rtMgr.ListSandboxes()),
			}
			hbMsg, err := protocol.NewMessage(protocol.MsgTypeHeartbeat, hbPayload)
			if err == nil {
				_ = a.sendMessage(hbMsg)
			}
		}
	}
}

func (a *Agent) dispatchMessage(msg *protocol.Message) {
	switch msg.Type {
	case protocol.MsgTypeRegisterAck:
		log.Printf("[AGENT] Registration acknowledged by Control Hub")

	case protocol.MsgTypeHeartbeatAck:
		// Heartbeat acknowledged, all normal

	case protocol.MsgTypeCreateBox:
		a.handleCreateBox(msg)

	case protocol.MsgTypeDestroyBox:
		a.handleDestroyBox(msg)

	default:
		log.Printf("[AGENT] Unknown message type: %s", msg.Type)
	}
}

// handleCreateBox applies the fine-grained SandboxPolicy sent by Control Hub!
func (a *Agent) handleCreateBox(msg *protocol.Message) {
	var req protocol.CreateBoxReq
	if err := msg.UnpackPayload(&req); err != nil {
		log.Printf("[AGENT] Unpack CreateBoxReq failed: %v", err)
		return
	}

	log.Printf("[AGENT] Received CreateBox for %s (User %d, Slot %d)", req.SandboxID, req.UserID, req.SlotIndex)

	// Merge Control fine-grained policy with local defaults
	limits := a.resolveResourceLimits(req.Policy.Resources)

	tmpSize := req.Policy.Storage.TmpfsTmpSize
	if tmpSize == "" {
		tmpSize = a.cfg.Runtime.Tmpfs.TmpSize
	}
	runSize := req.Policy.Storage.TmpfsRunSize
	if runSize == "" {
		runSize = a.cfg.Runtime.Tmpfs.RunSize
	}

	// 1. Provision btrfs snapshot & Cgroups v2 with fine-grained limits
	sbx, err := a.rtMgr.CreateSandbox(context.Background(), rt.SandboxConfig{
		ID:           req.SandboxID,
		Limits:       limits,
		TmpfsTmpSize: tmpSize,
		TmpfsRunSize: runSize,
	})

	respPayload := protocol.CreateBoxResp{
		SandboxID: req.SandboxID,
	}

	if err != nil {
		log.Printf("[AGENT] CreateSandbox %s failed: %v", req.SandboxID, err)
		respPayload.Success = false
		respPayload.Error = err.Error()
	} else {
		// 2. Setup isolated network if slot index is valid
		slotIdx := req.SlotIndex
		if slotIdx <= 0 {
			slotIdx = 1
		}
		netns := fmt.Sprintf("/proc/%d/ns/net", sbx.PID)
		sbxNet, netErr := network.SetupSandboxNetwork(context.Background(), a.cfg.Network.BridgeName, slotIdx, netns)
		if netErr != nil {
			log.Printf("[AGENT] SetupSandboxNetwork %s failed: %v", req.SandboxID, netErr)
			// Network failure: teardown container and report error
			_ = a.rtMgr.DestroySandbox(context.Background(), req.SandboxID)
			respPayload.Success = false
			respPayload.Error = netErr.Error()
		} else {
			a.slotsMu.Lock()
			a.slots[req.SandboxID] = slotIdx
			a.slotsMu.Unlock()

			respPayload.Success = true
			respPayload.SandboxIP = sbxNet.IP
			respPayload.TargetIP = "172.17.0.2" // Will be dynamically mapped from target runner
			log.Printf("[AGENT] Sandbox %s created successfully! (IP: %s, Slot: %d)", req.SandboxID, sbxNet.IP, slotIdx)
		}
	}

	// Send RPC response
	respMsg, err := protocol.NewMessage(protocol.MsgTypeCreateBoxResp, respPayload)
	if err == nil {
		respMsg.ID = msg.ID // Match request ID for RPC correlation
		_ = a.sendMessage(respMsg)
	}
}

func (a *Agent) handleDestroyBox(msg *protocol.Message) {
	var req protocol.DestroyBoxReq
	if err := msg.UnpackPayload(&req); err != nil {
		log.Printf("[AGENT] Unpack DestroyBoxReq failed: %v", err)
		return
	}

	log.Printf("[AGENT] Received DestroyBox for %s (Reason: %s)", req.SandboxID, req.Reason)

	// Clean up network veth first if tracked
	a.slotsMu.Lock()
	slotIdx := a.slots[req.SandboxID]
	delete(a.slots, req.SandboxID)
	a.slotsMu.Unlock()

	if slotIdx > 0 {
		_ = network.TeardownSandboxNetwork(context.Background(), slotIdx)
	}

	err := a.rtMgr.DestroySandbox(context.Background(), req.SandboxID)

	respPayload := protocol.DestroyBoxResp{
		SandboxID: req.SandboxID,
		Success:   err == nil,
	}
	if err != nil {
		respPayload.Error = err.Error()
	}

	respMsg, err := protocol.NewMessage(protocol.MsgTypeDestroyBoxResp, respPayload)
	if err == nil {
		respMsg.ID = msg.ID
		_ = a.sendMessage(respMsg)
	}
}

func (a *Agent) resolveResourceLimits(quota protocol.BoxResourceQuota) rt.ResourceLimits {
	// Start with worker config defaults
	resolved := rt.LimitsFromConfig(a.cfg.Runtime.Limits)

	// Apply Control overrides if specified (> 0)
	if quota.PIDsMax > 0 {
		resolved.PIDsMax = quota.PIDsMax
	}
	if quota.MemoryMaxMB > 0 {
		resolved.MemoryMaxBytes = quota.MemoryMaxMB * 1024 * 1024
	}
	if quota.MemorySwapMaxMB >= 0 {
		resolved.SwapMaxBytes = quota.MemorySwapMaxMB * 1024 * 1024
	}
	if quota.CPUWeight > 0 {
		resolved.CPUWeight = quota.CPUWeight
	}
	if quota.IOWriteBytesSec > 0 {
		resolved.IOWriteBytesSec = quota.IOWriteBytesSec
	}
	if quota.IOWriteIOPS > 0 {
		resolved.IOWriteIOPS = quota.IOWriteIOPS
	}

	return resolved
}

func hasKVMSupport() bool {
	if _, err := os.Stat("/dev/kvm"); err == nil {
		return true
	}
	return false
}
