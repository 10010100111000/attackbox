package node

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"

	rt "attackbox/node/runtime"
	"attackbox/protocol"
)

// -------------------------------------------------------------
// Node Configuration (Parsed directly from configs/node.yaml)
// -------------------------------------------------------------

type NodeInfoConfig struct {
	NodeID               string `yaml:"node_id"`
	ClusterSecret        string `yaml:"cluster_secret"`
	ControlURL           string `yaml:"control_url"`
	HeartbeatIntervalSec int    `yaml:"heartbeat_interval_sec"`
}

type NodeConfig struct {
	Node    NodeInfoConfig   `yaml:"node"`
	Gateway GatewayConfig    `yaml:"gateway"`
	Runtime rt.RuntimeConfig `yaml:"runtime"`
	Network NetworkConfig    `yaml:"network"`
}

// LoadNodeConfig parses configs/node.yaml or falls back to production defaults.
func LoadNodeConfig(path string) (*NodeConfig, error) {
	cfg := &NodeConfig{
		Node: NodeInfoConfig{
			NodeID:               "node-primary",
			ClusterSecret:        "attackbox_cluster_secret_change_me_in_prod!",
			ControlURL:           "ws://127.0.0.1:18080/api/agent/connect",
			HeartbeatIntervalSec: 3,
		},
		Gateway: GatewayConfig{
			ListenAddr: "0.0.0.0:8443",
			TLSEnabled: false,
		},
		Runtime: rt.RuntimeConfig{
			BaseRootfs:   "/var/lib/attackbox/base_rootfs",
			InstancesDir: "/var/lib/attackbox/instances",
			CgroupRoot:   "/sys/fs/cgroup/attackbox",
			Limits: rt.RuntimeLimitsConfig{
				PIDsMax:         256,
				MemoryMaxMB:     1024,
				MemorySwapMaxMB: 0,
				CPUWeight:       100,
				IOWriteBytesSec: 20971520,
				IOWriteIOPS:     1000,
			},
			DiskGuard: rt.DiskGuardConfig{
				Enabled:          true,
				CheckIntervalSec: 3,
				MinFreeSpaceMB:   5120,
				WarnFreeSpaceMB:  10240,
			},
			Tmpfs: rt.TmpfsConfig{
				TmpSize: "512M",
				RunSize: "256M",
			},
		},
		Network: NetworkConfig{
			BridgeName: "br-attackbox",
			GatewayIP:  "10.200.0.1",
			SubnetCIDR: "10.200.0.0/16",
			Interface:  "eth0",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read node config %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}
	return cfg, nil
}

// -------------------------------------------------------------
// Node Control Client (Reverse WebSocket to Control Hub)
// -------------------------------------------------------------

type sandboxMailbox struct {
	ch chan *protocol.Message
}

type NodeControlClient struct {
	cfg         *NodeConfig
	rtMgr       *rt.RuntimeManager
	conn        *websocket.Conn
	connMu      sync.Mutex
	bootID      string
	seq         uint64
	stopCh      chan struct{}
	wg          sync.WaitGroup
	onKeySync   func(pubKey ed25519.PublicKey)
	mailboxesMu sync.Mutex
	mailboxes   map[string]*sandboxMailbox
}

func NewNodeControlClient(cfg *NodeConfig, rtMgr *rt.RuntimeManager, onKeySync func(pubKey ed25519.PublicKey)) *NodeControlClient {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	bootID := hex.EncodeToString(b)
	return &NodeControlClient{
		cfg:       cfg,
		rtMgr:     rtMgr,
		bootID:    bootID,
		stopCh:    make(chan struct{}),
		onKeySync: onKeySync,
		mailboxes: make(map[string]*sandboxMailbox),
	}
}

func (c *NodeControlClient) Start(ctx context.Context) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.connectLoop(ctx)
	}()
}

func (c *NodeControlClient) Stop() {
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
		hb, _ := protocol.NewMessage(protocol.MsgTypeHeartbeat, protocol.HeartbeatReq{NodeID: c.cfg.Node.NodeID, BootID: c.bootID, Seq: atomic.AddUint64(&c.seq, 1), InventoryRevision: revision, Timestamp: time.Now().Unix(), LoadAvg: readSystemLoadAvg(), MemAvailableMB: available, DiskFreeGB: c.rtMgr.DiskGuard().GetCurrentFreeMB() / 1024, ActiveSandboxes: len(inventory), Sandboxes: inventory})
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

func (c *NodeControlClient) DispatchMessageForTest(msg *protocol.Message) {
	c.dispatchMessage(msg)
}

func (c *NodeControlClient) dispatchMessage(msg *protocol.Message) {
	switch msg.Type {
	case protocol.MsgTypeRegisterAck:
		log.Printf("[CLUSTER] Registration acknowledged by Control Hub")
		var resp protocol.RegisterResp
		if err := msg.UnpackPayload(&resp); err == nil && resp.PublicKey != "" {
			if keyBytes, err := hex.DecodeString(resp.PublicKey); err == nil && len(keyBytes) == ed25519.PublicKeySize {
				if c.onKeySync != nil {
					c.onKeySync(ed25519.PublicKey(keyBytes))
					log.Printf("[CLUSTER] Control Hub Ed25519 public key dynamically synchronized (%d bytes)", len(keyBytes))
				}
			}
		}
	case protocol.MsgTypeHeartbeatAck:
		// Heartbeat acknowledged

	case protocol.MsgTypeCreateBox, protocol.MsgTypeDestroyBox:
		var hdr struct {
			SandboxID string `json:"sandbox_id"`
		}
		if err := msg.UnpackPayload(&hdr); err != nil || hdr.SandboxID == "" {
			log.Printf("[CLUSTER] Failed to unpack sandbox_id from %s: %v", msg.Type, err)
			c.sendErrorResponse(msg, "missing or invalid sandbox_id")
			return
		}

		c.mailboxesMu.Lock()
		mb, exists := c.mailboxes[hdr.SandboxID]
		if !exists {
			mb = &sandboxMailbox{
				ch: make(chan *protocol.Message, 4),
			}
			c.mailboxes[hdr.SandboxID] = mb
			go c.runSandboxMailbox(hdr.SandboxID, mb)
		}

		select {
		case mb.ch <- msg:
			c.mailboxesMu.Unlock()
		default:
			c.mailboxesMu.Unlock()
			log.Printf("[ERROR] Mailbox for sandbox %s full (capacity 4), rejecting %s", hdr.SandboxID, msg.Type)
			c.sendErrorResponse(msg, "sandbox operation queue is full")
		}

	default:
		log.Printf("[CLUSTER] Unknown message type: %s", msg.Type)
	}
}

func (c *NodeControlClient) runSandboxMailbox(sandboxID string, mb *sandboxMailbox) {
	for {
		select {
		case <-c.stopCh:
			return
		case msg, ok := <-mb.ch:
			if !ok {
				return
			}
			switch msg.Type {
			case protocol.MsgTypeCreateBox:
				c.handleCreateBox(msg)
			case protocol.MsgTypeDestroyBox:
				c.handleDestroyBox(msg)
				c.mailboxesMu.Lock()
				if len(mb.ch) == 0 {
					delete(c.mailboxes, sandboxID)
					c.mailboxesMu.Unlock()
					return
				}
				c.mailboxesMu.Unlock()
			}
		}
	}
}

func (c *NodeControlClient) sendErrorResponse(msg *protocol.Message, errMsg string) {
	var respMsg *protocol.Message
	switch msg.Type {
	case protocol.MsgTypeCreateBox:
		var req protocol.CreateBoxReq
		_ = msg.UnpackPayload(&req)
		respMsg, _ = protocol.NewMessage(protocol.MsgTypeCreateBoxResp, protocol.CreateBoxResp{
			SandboxID: req.SandboxID,
			Success:   false,
			Error:     errMsg,
		})
	case protocol.MsgTypeDestroyBox:
		var req protocol.DestroyBoxReq
		_ = msg.UnpackPayload(&req)
		respMsg, _ = protocol.NewMessage(protocol.MsgTypeDestroyBoxResp, protocol.DestroyBoxResp{
			SandboxID: req.SandboxID,
			Success:   false,
			Error:     errMsg,
		})
	default:
		return
	}
	if respMsg != nil {
		respMsg.ID = msg.ID
		_ = c.sendMessage(respMsg)
	}
}

func (c *NodeControlClient) handleCreateBox(msg *protocol.Message) {
	var req protocol.CreateBoxReq
	if err := msg.UnpackPayload(&req); err != nil {
		log.Printf("[CLUSTER] Unpack CreateBoxReq failed: %v", err)
		c.sendErrorResponse(msg, fmt.Sprintf("unpack request: %v", err))
		return
	}

	if req.BootID != c.bootID {
		c.sendErrorResponse(msg, "worker generation changed")
		return
	}

	log.Printf("[CLUSTER] Received CreateBox for %s (User %d, Slot %d)", req.SandboxID, req.UserID, req.SlotIndex)

	if c.rtMgr == nil {
		c.sendErrorResponse(msg, "runtime unavailable")
		return
	}

	// Fallback to node config defaults if unspecified
	if req.Policy.Storage.TmpfsTmpSize == "" && c.cfg.Runtime.Tmpfs.TmpSize != "" {
		req.Policy.Storage.TmpfsTmpSize = c.cfg.Runtime.Tmpfs.TmpSize
	}
	if req.Policy.Storage.TmpfsRunSize == "" && c.cfg.Runtime.Tmpfs.RunSize != "" {
		req.Policy.Storage.TmpfsRunSize = c.cfg.Runtime.Tmpfs.RunSize
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	sbx, err := c.rtMgr.CreateSandbox(ctx, rt.SandboxConfig{
		TTLSeconds: req.TTLSeconds, IdleSeconds: req.IdleSeconds,
		ID:        req.SandboxID,
		UserID:    req.UserID,
		SlotIndex: req.SlotIndex,
		Policy:    req.Policy,
	})

	respPayload := protocol.CreateBoxResp{
		SandboxID: req.SandboxID,
		BootID:    c.bootID,
		Success:   err == nil,
	}
	if err != nil {
		respPayload.Error = err.Error()
		log.Printf("[CLUSTER] CreateSandbox %s failed: %v", req.SandboxID, err)
	} else {
		respPayload.SandboxIP = sbx.IP
		respPayload.ExpiresAt = sbx.ExpiresAt
		log.Printf("[CLUSTER] Sandbox %s created successfully! (IP: %s, Slot: %d)", req.SandboxID, sbx.IP, sbx.SlotIndex)
	}

	respPayload.InventoryRevision, _ = c.rtMgr.Inventory()
	respMsg, packErr := protocol.NewMessage(protocol.MsgTypeCreateBoxResp, respPayload)
	if packErr == nil {
		respMsg.ID = msg.ID
		_ = c.sendMessage(respMsg)
	}
}

func (c *NodeControlClient) handleDestroyBox(msg *protocol.Message) {
	var req protocol.DestroyBoxReq
	if err := msg.UnpackPayload(&req); err != nil {
		log.Printf("[CLUSTER] Unpack DestroyBoxReq failed: %v", err)
		c.sendErrorResponse(msg, fmt.Sprintf("unpack request: %v", err))
		return
	}

	if req.BootID != c.bootID {
		c.sendErrorResponse(msg, "worker generation changed")
		return
	}

	log.Printf("[CLUSTER] Received DestroyBox for %s (Reason: %s)", req.SandboxID, req.Reason)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var err error
	if c.rtMgr == nil {
		c.sendErrorResponse(msg, "runtime unavailable")
		return
	}
	if c.rtMgr != nil {
		err = c.rtMgr.DestroySandbox(ctx, req.SandboxID)
	}

	respPayload := protocol.DestroyBoxResp{
		SandboxID: req.SandboxID,
		BootID:    c.bootID,
		Success:   err == nil,
	}
	if err != nil {
		respPayload.Error = err.Error()
		log.Printf("[CLUSTER] DestroySandbox %s failed: %v", req.SandboxID, err)
	} else {
		log.Printf("[CLUSTER] DestroySandbox %s succeeded", req.SandboxID)
	}

	respMsg, packErr := protocol.NewMessage(protocol.MsgTypeDestroyBoxResp, respPayload)
	if packErr == nil {
		respMsg.ID = msg.ID
		_ = c.sendMessage(respMsg)
	}
}

func hasKVMSupport() bool {
	if _, err := os.Stat("/dev/kvm"); err == nil {
		return true
	}
	return false
}

// readSystemMemoryMB reads real Total and Available physical memory from /proc/meminfo.
func readSystemMemoryMB() (totalMB int64, availableMB int64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				totalMB = kb / 1024
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				availableMB = kb / 1024
			}
		}
		if totalMB > 0 && availableMB > 0 {
			break
		}
	}
	return totalMB, availableMB
}

// readSystemLoadAvg reads real 1-minute system load average from /proc/loadavg.
func readSystemLoadAvg() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0.0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		val, _ := strconv.ParseFloat(fields[0], 64)
		return val
	}
	return 0.0
}
