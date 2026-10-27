package node

import (
	rt "attackbox/node/runtime"
	"attackbox/protocol"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type NodeControlClient struct {
	compose          *composeLabs
	composeInitError string
	workCancel       context.CancelFunc
	cfg              *NodeConfig
	rtMgr            *rt.RuntimeManager
	conn             *websocket.Conn
	connMu           sync.Mutex
	bootID           string
	seq              uint64
	stopCh           chan struct{}
	wg               sync.WaitGroup
	onKeySync        func(pubKey ed25519.PublicKey)
	mailboxesMu      sync.Mutex
	mailboxes        map[string]*sandboxMailbox
}

func NewNodeControlClient(cfg *NodeConfig, rtMgr *rt.RuntimeManager, onKeySync func(pubKey ed25519.PublicKey)) *NodeControlClient {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	bootID := hex.EncodeToString(b)
	client := &NodeControlClient{
		cfg:       cfg,
		rtMgr:     rtMgr,
		bootID:    bootID,
		stopCh:    make(chan struct{}),
		onKeySync: onKeySync,
		mailboxes: make(map[string]*sandboxMailbox),
	}
	if cfg.ComposeLabs.Enabled {
		var err error
		client.compose, err = newComposeLabs(cfg, rtMgr)
		if err != nil {
			client.composeInitError = "compose_initialization_failed"
			log.Printf("[COMPOSE] initialization failed: %v", err)
		} else if rtMgr != nil {
			rtMgr.SetLabLifecycle(client.compose.runtime.SandboxAvailable, client.compose.runtime.SandboxGone)
		}
	}
	return client
}

func (c *NodeControlClient) Start(ctx context.Context) {
	targetCtx, cancel := context.WithCancel(ctx)
	c.workCancel = cancel
	if c.compose != nil {
		c.wg.Add(1)
		go func() { defer c.wg.Done(); c.experimentReaper(targetCtx) }()
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.connectLoop(ctx)
	}()
}

func (c *NodeControlClient) Stop() {
	if c.workCancel != nil {
		c.workCancel()
	}
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}

	c.connMu.Lock()
	if c.conn != nil {
		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "node shutting down"),
			time.Now().Add(time.Second),
		)
		_ = c.conn.Close()
	}
	c.connMu.Unlock()

	c.wg.Wait()
	if c.compose != nil {
		if c.rtMgr != nil {
			c.rtMgr.SetLabLifecycle(nil, nil)
		}
		c.compose.Close()
	}
}

func (c *NodeControlClient) connectLoop(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 15 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		err := c.runSession(ctx)
		if err != nil {
			log.Printf("[CLUSTER] Control channel disconnected: %v (retrying in %v)", err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (c *NodeControlClient) runSession(ctx context.Context) error {
	controlURL := c.cfg.Node.ControlURL
	if controlURL == "" {
		return fmt.Errorf("control_url is empty")
	}

	u, err := url.Parse(controlURL)
	if err != nil {
		return fmt.Errorf("invalid control_url: %w", err)
	}

	headers := http.Header{}
	if c.cfg.Node.ClusterSecret != "" {
		headers.Set("Authorization", "Bearer "+c.cfg.Node.ClusterSecret)
	}
	headers.Set("X-Node-ID", c.cfg.Node.NodeID)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("dial control hub: %w", err)
	}
	conn.SetReadLimit(protocol.MaxWebSocketMessageSize)

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
	defer func() {
		c.connMu.Lock()
		if c.conn == conn {
			_ = conn.Close()
			c.conn = nil
		}
		c.connMu.Unlock()
	}()

	log.Printf("[CLUSTER] Connected to Control Hub at %s", controlURL)

	dataPort := 8443
	if c.cfg.Gateway.ListenAddr != "" {
		_, portStr, err := net.SplitHostPort(c.cfg.Gateway.ListenAddr)
		if err == nil {
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				dataPort = p
			}
		} else {
			portStr := strings.TrimPrefix(c.cfg.Gateway.ListenAddr, ":")
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				dataPort = p
			}
		}
	}

	totalMemMB, _ := readSystemMemoryMB()

	activeIDs := sandboxIDs(c.rtMgr)

	// 1. Send Registration frame
	regPayload := protocol.RegisterReq{
		NodeID:           c.cfg.Node.NodeID,
		BootID:           c.bootID,
		Version:          "2.0",
		DataPort:         dataPort,
		PublicDataURL:    c.cfg.Gateway.PublicDataURL,
		ActiveSandboxIDs: activeIDs,
		Capabilities: protocol.NodeCapabilities{
			HasKVM:     hasKVMSupport(),
			CPUCores:   runtime.NumCPU(),
			TotalMemMB: totalMemMB,
			OSVersion:  runtime.GOOS + "/" + runtime.GOARCH,
		},
	}

	regMsg, err := protocol.NewMessage(protocol.MsgTypeRegister, regPayload)
	if err != nil {
		return fmt.Errorf("create register message: %w", err)
	}
	if err := c.sendMessage(regMsg); err != nil {
		return fmt.Errorf("send register message: %w", err)
	}

	// 2. Start periodic heartbeat goroutine
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	go c.heartbeatLoop(sessionCtx)

	// 3. Receive & Dispatch loop
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read control message: %w", err)
		}

		msg := new(protocol.Message)
		if err := protocol.UnpackMessage(rawMsg, msg); err != nil {
			log.Printf("[CLUSTER] Failed to parse message frame: %v", err)
			continue
		}

		c.dispatchMessage(msg)
	}
}

func (c *NodeControlClient) sendMessage(msg *protocol.Message) error {
	data, err := protocol.PackMessage(msg)
	if err != nil {
		return fmt.Errorf("pack message: %w", err)
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("connection closed")
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func sandboxIDs(m *rt.RuntimeManager) []string {
	if m == nil {
		return nil
	}
	_, inventory := m.Inventory()
	ids := make([]string, 0, len(inventory))
	for _, s := range inventory {
		ids = append(ids, s.ID)
	}
	return ids
}

func (c *NodeControlClient) heartbeatLoop(ctx context.Context) {
	interval := time.Duration(c.cfg.Node.HeartbeatIntervalSec) * time.Second
	if interval <= 0 {
		interval = 3 * time.Second
	}
	send := func() {
		_, available := readSystemMemoryMB()
		revision, inventory := c.rtMgr.Inventory()
		var compose *protocol.ComposeCapability
		if c.cfg.ComposeLabs.Enabled {
			compose = &protocol.ComposeCapability{Available: c.compose != nil, NetworkVerified: c.cfg.ComposeLabs.NetworkVerified, ProtocolVersion: 2, Error: c.composeInitError}
		}
		hb, _ := protocol.NewMessage(protocol.MsgTypeHeartbeat, protocol.HeartbeatReq{ComposeLabs: compose, NodeID: c.cfg.Node.NodeID, BootID: c.bootID, Seq: atomic.AddUint64(&c.seq, 1), InventoryRevision: revision, Timestamp: time.Now().Unix(), LoadAvg: readSystemLoadAvg(), MemAvailableMB: available, DiskFreeGB: c.rtMgr.DiskGuard().GetCurrentFreeMB() / 1024, ActiveSandboxes: len(inventory), Sandboxes: inventory})
		_ = c.sendMessage(hb)
	}
	send()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			send()
		}
	}
}
