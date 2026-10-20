package control

import (
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/protocol"
)

var (
	ErrNoWorkersAvailable = errors.New("no active worker nodes available in cluster")
	ErrWorkerNotFound     = errors.New("worker node not found")
	ErrRPCTimeout         = errors.New("worker RPC request timed out")
)

// WorkerNode represents an active sacrificial worker connection in the cluster pool.
type WorkerNode struct {
	NodeID          string
	RemoteIP        string

	stateMu         sync.RWMutex
	State           protocol.NodeState
	DataPort        int
	PublicDataURL   string
	Capabilities    protocol.NodeCapabilities
	Telemetry       protocol.HeartbeatReq
	ActiveSandboxes int
	LastHeartbeat   time.Time

	conn            *websocket.Conn
	connMu          sync.Mutex
	pendingMu       sync.Mutex
	pendingRPC      map[string]chan *protocol.Message
}

func (w *WorkerNode) SetState(s protocol.NodeState) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.State = s
}

func (w *WorkerNode) GetState() protocol.NodeState {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.State
}

func (w *WorkerNode) UpdateRegister(reg protocol.RegisterReq) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.State = protocol.NodeStateOnline
	w.Capabilities = reg.Capabilities
	if reg.DataPort > 0 {
		w.DataPort = reg.DataPort
	}
	if reg.PublicDataURL != "" {
		w.PublicDataURL = reg.PublicDataURL
	}
}

func (w *WorkerNode) GetDataURL(fallbackHost string) string {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	if w.PublicDataURL != "" {
		return strings.TrimSuffix(w.PublicDataURL, "/") + "/ws/cli"
	}
	host := w.RemoteIP
	if host == "" || host == "127.0.0.1" {
		host = fallbackHost
	}
	port := w.DataPort
	if port <= 0 {
		port = 8443
	}
	return fmt.Sprintf("ws://%s:%d/ws/cli", host, port)
}

func (w *WorkerNode) UpdateHeartbeat(hb protocol.HeartbeatReq) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.LastHeartbeat = time.Now()
	w.Telemetry = hb
	w.ActiveSandboxes = hb.ActiveSandboxes
	// If node was marked offline due to missed heartbeats, restore to Online
	if w.State == protocol.NodeStateOffline || w.State == "" {
		w.State = protocol.NodeStateOnline
	}
}

func (w *WorkerNode) GetSchedulingSnapshot() (state protocol.NodeState, lastHB time.Time, activeSandboxes int, dataPort int) {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.State, w.LastHeartbeat, w.ActiveSandboxes, w.DataPort
}

func (w *WorkerNode) Send(msg *protocol.Message) error {
	data, err := protocol.PackMessage(msg)
	if err != nil {
		return err
	}
	w.connMu.Lock()
	defer w.connMu.Unlock()
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

// NodeClusterManager manages active worker WebSocket connections and coordinates RPC dispatching.
type NodeClusterManager struct {
	clusterSecret       string
	pubKeyHex           string
	heartbeatTimeoutSec int
	rpcTimeoutSec       int
	reconcileHandler    func(workerID string, sandboxes []protocol.SandboxReconcileInfo) protocol.ReconcileResp
	mu                  sync.RWMutex
	workers             map[string]*WorkerNode
	upgrader            websocket.Upgrader
}

// SetReconcileHandler registers a handler triggered when a worker reports state reconciliation.
func (h *NodeClusterManager) SetReconcileHandler(fn func(workerID string, sandboxes []protocol.SandboxReconcileInfo) protocol.ReconcileResp) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reconcileHandler = fn
}

// SetNodeState transitions a worker node's lifecycle state (Online, Draining, Offline, Resetting).
func (h *NodeClusterManager) SetNodeState(nodeID string, state protocol.NodeState) error {
	h.mu.RLock()
	w, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return ErrWorkerNotFound
	}
	w.SetState(state)
	return nil
}

// GetNodeState returns a worker node's current lifecycle state.
func (h *NodeClusterManager) GetNodeState(nodeID string) (protocol.NodeState, error) {
	h.mu.RLock()
	w, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return protocol.NodeStateOffline, ErrWorkerNotFound
	}
	return w.GetState(), nil
}

func (h *NodeClusterManager) GetWorkerDataURL(nodeID, fallbackHost string) (string, bool) {
	h.mu.RLock()
	node, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return "", false
	}
	return node.GetDataURL(fallbackHost), true
}

// NewNodeClusterManager creates a cluster manager.
func NewNodeClusterManager(clusterSecret string) *NodeClusterManager {
	return &NodeClusterManager{
		clusterSecret:       clusterSecret,
		heartbeatTimeoutSec: 15,
		rpcTimeoutSec:       30,
		workers:             make(map[string]*WorkerNode),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1 << 15,
			WriteBufferSize: 1 << 15,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // Legitimate worker Go daemon does not send Origin header
				}
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				return strings.EqualFold(u.Host, r.Host)
			},
		},
	}
}

// SetTimeouts dynamically updates the heartbeat and RPC timeouts from config.
func (h *NodeClusterManager) SetTimeouts(hbSec, rpcSec int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if hbSec > 0 {
		h.heartbeatTimeoutSec = hbSec
	}
	if rpcSec > 0 {
		h.rpcTimeoutSec = rpcSec
	}
}

// SetPublicKey sets the Ed25519 public key distributed to worker nodes on registration.
func (h *NodeClusterManager) SetPublicKey(pubKey ed25519.PublicKey) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pubKeyHex = hex.EncodeToString(pubKey)
}

// HandleAgentConnect handles reverse WebSocket connection handshakes from Worker agents.
func (h *NodeClusterManager) HandleAgentConnect(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate cluster_secret in constant time to prevent Timing Attacks
	authHeader := r.Header.Get("Authorization")
	secret := strings.TrimPrefix(authHeader, "Bearer ")
	if secret == "" {
		secret = r.URL.Query().Get("secret")
	}

	if subtle.ConstantTimeCompare([]byte(secret), []byte(h.clusterSecret)) != 1 {
		http.Error(w, "invalid cluster secret", http.StatusUnauthorized)
		return
	}

	nodeID := r.Header.Get("X-Node-ID")
	if nodeID == "" {
		nodeID = r.URL.Query().Get("node_id")
	}
	if nodeID == "" {
		http.Error(w, "missing node ID", http.StatusBadRequest)
		return
	}

	// Extract remote IP
	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		remoteIP = strings.Split(fwd, ",")[0]
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[CLUSTER] WebSocket upgrade failed: %v", err)
		return
	}
	conn.SetReadLimit(protocol.MaxWebSocketMessageSize)

	node := &WorkerNode{
		NodeID:        nodeID,
		RemoteIP:      remoteIP,
		DataPort:      8443,
		State:         protocol.NodeStateOnline,
		LastHeartbeat: time.Now(),
		conn:          conn,
		pendingRPC:    make(map[string]chan *protocol.Message),
	}

	h.registerWorker(node)
	defer h.unregisterWorker(node)

	log.Printf("[CLUSTER] Node %s connected from %s", nodeID, remoteIP)

	// Inbound message loop
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[CLUSTER] Node %s disconnected: %v", nodeID, err)
			break
		}

		var msg protocol.Message
		if err := protocol.UnpackMessage(raw, &msg); err != nil {
			continue
		}

		h.handleWorkerMessage(node, &msg)
	}
}

func (h *NodeClusterManager) registerWorker(w *WorkerNode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// If a worker with the same nodeID was previously connected, close the stale connection
	// to prevent orphaned goroutines and avoid leaking old RPC loops.
	if old, exists := h.workers[w.NodeID]; exists && old != w && old.conn != nil {
		_ = old.conn.Close()
	}
	h.workers[w.NodeID] = w
}

func (h *NodeClusterManager) unregisterWorker(w *WorkerNode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// P1-5 Fix: Only unregister if the worker currently registered in the pool is this exact instance.
	// If the node has reconnected, h.workers[w.NodeID] will already point to the new WorkerNode instance,
	// and unregistering by nodeID alone would inadvertently kick the newly reconnected worker out of the cluster!
	if current, exists := h.workers[w.NodeID]; exists && current == w {
		delete(h.workers, w.NodeID)
		log.Printf("[CLUSTER] Node %s unregistered (session terminated)", w.NodeID)
	}
}

func (h *NodeClusterManager) handleWorkerMessage(node *WorkerNode, msg *protocol.Message) {
	// Check if this message is a response to a pending RPC
	node.pendingMu.Lock()
	ch, isResponse := node.pendingRPC[msg.ID]
	if isResponse {
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		select {
		case ch <- msg:
		default:
		}
		return
	}
	node.pendingMu.Unlock()

	// Handle spontaneous worker messages (Register, Heartbeat)
	switch msg.Type {
	case protocol.MsgTypeRegister:
		var reg protocol.RegisterReq
		if err := msg.UnpackPayload(&reg); err == nil {
			node.UpdateRegister(reg)
			// Send RegisterAck with distributed Control Hub Ed25519 public key
			h.mu.RLock()
			pk := h.pubKeyHex
			h.mu.RUnlock()
			ack, _ := protocol.NewMessage(protocol.MsgTypeRegisterAck, protocol.RegisterResp{
				Success:   true,
				NodeID:    node.NodeID,
				PublicKey: pk,
			})
			_ = node.Send(ack)
			log.Printf("[CLUSTER] Registered Node %s (CPUs: %d, DataPort: %d)", node.NodeID, reg.Capabilities.CPUCores, reg.DataPort)
		}

	case protocol.MsgTypeHeartbeat:
		var hb protocol.HeartbeatReq
		if err := msg.UnpackPayload(&hb); err == nil {
			node.UpdateHeartbeat(hb)
			// Send HeartbeatAck
			ack, _ := protocol.NewMessage(protocol.MsgTypeHeartbeatAck, protocol.HeartbeatResp{Success: true})
			_ = node.Send(ack)
		}

	case protocol.MsgTypeReconcile:
		var req protocol.ReconcileReq
		if err := msg.UnpackPayload(&req); err == nil {
			h.mu.RLock()
			handler := h.reconcileHandler
			h.mu.RUnlock()
			var resp protocol.ReconcileResp
			if handler != nil {
				resp = handler(node.NodeID, req.Sandboxes)
			} else {
				resp = protocol.ReconcileResp{Success: true}
			}
			respMsg, _ := protocol.NewMessage(protocol.MsgTypeReconcileResp, resp)
			respMsg.ID = msg.ID
			_ = node.Send(respMsg)
		}

	case protocol.MsgTypeCreateBoxResp:
		// P1-9: Late response from a CreateBox that timed out or was canceled on Control Hub.
		// If Worker succeeded in creating the box after Control abandoned it, issue immediate reverse kill!
		var resp protocol.CreateBoxResp
		if err := msg.UnpackPayload(&resp); err == nil && resp.Success && resp.SandboxID != "" {
			log.Printf("[CLUSTER] Received late/orphaned CreateBoxResp for %s from worker %s. Issuing reverse destroy!", resp.SandboxID, node.NodeID)
			go func(workerID, sID string) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				_ = h.DestroyBox(ctx, workerID, protocol.DestroyBoxReq{
					SandboxID: sID,
					Reason:    "late_create_response_rollback",
				})
			}(node.NodeID, resp.SandboxID)
		}
	}
}

// SelectBestWorker applies least-connections scheduling across active workers.
func (h *NodeClusterManager) SelectBestWorker() (*WorkerNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.workers) == 0 {
		return nil, ErrNoWorkersAvailable
	}

	var best *WorkerNode
	minLoad := int(^uint(0) >> 1) // max int
	now := time.Now()

	for _, w := range h.workers {
		state, lastHB, activeBoxes, _ := w.GetSchedulingSnapshot()
		hbTimeout := 15 * time.Second
		if h.heartbeatTimeoutSec > 0 {
			hbTimeout = time.Duration(h.heartbeatTimeoutSec) * time.Second
		}
		if now.Sub(lastHB) > hbTimeout {
			w.SetState(protocol.NodeStateOffline)
			continue
		}
		if state != protocol.NodeStateOnline {
			continue
		}
		if activeBoxes < minLoad {
			minLoad = activeBoxes
			best = w
		}
	}

	if best == nil {
		return nil, ErrNoWorkersAvailable
	}
	return best, nil
}

// GetWorkerEndpoint retrieves the RemoteIP and DataPort of a specific worker node.
func (h *NodeClusterManager) GetWorkerEndpoint(nodeID string) (remoteIP string, dataPort int, exists bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	w, ok := h.workers[nodeID]
	if !ok {
		return "", 0, false
	}
	_, _, _, dataPort = w.GetSchedulingSnapshot()
	return w.RemoteIP, dataPort, true
}

// CreateBox dispatches a CreateBox RPC with fine-grained policy to the target worker.
func (h *NodeClusterManager) CreateBox(ctx context.Context, nodeID string, req protocol.CreateBoxReq) (*protocol.CreateBoxResp, error) {
	h.mu.RLock()
	node, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return nil, ErrWorkerNotFound
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeCreateBox, req)
	if err != nil {
		return nil, err
	}

	respChan := make(chan *protocol.Message, 1)
	node.pendingMu.Lock()
	node.pendingRPC[msg.ID] = respChan
	node.pendingMu.Unlock()

	if err := node.Send(msg); err != nil {
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return nil, err
	}

	rpcTimeout := 30 * time.Second
	if h.rpcTimeoutSec > 0 {
		rpcTimeout = time.Duration(h.rpcTimeoutSec) * time.Second
	}

	select {
	case <-ctx.Done():
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(rpcTimeout):
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return nil, ErrRPCTimeout
	case respMsg := <-respChan:
		var resp protocol.CreateBoxResp
		if err := respMsg.UnpackPayload(&resp); err != nil {
			return nil, fmt.Errorf("unpack response: %w", err)
		}
		if !resp.Success {
			return nil, errors.New(resp.Error)
		}
		return &resp, nil
	}
}

// DestroyBox dispatches a DestroyBox RPC to the worker and waits for confirmation.
func (h *NodeClusterManager) DestroyBox(ctx context.Context, nodeID string, req protocol.DestroyBoxReq) error {
	h.mu.RLock()
	node, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		// P1-7: Return ErrWorkerNotFound when worker is not in pool.
		// Never return nil on missing worker, which falsely signals destruction success.
		return ErrWorkerNotFound
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeDestroyBox, req)
	if err != nil {
		return err
	}

	respChan := make(chan *protocol.Message, 1)
	node.pendingMu.Lock()
	node.pendingRPC[msg.ID] = respChan
	node.pendingMu.Unlock()

	if err := node.Send(msg); err != nil {
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return err
	}

	// P1-7: Avoid zero-value integer truncation pitfall (e.g. rpc_timeout_sec: 1 -> /2 -> 0s -> time.After(0))
	destroyTimeout := 15 * time.Second
	if h.rpcTimeoutSec > 0 {
		destroyTimeout = time.Duration(h.rpcTimeoutSec) * time.Second
	}
	if destroyTimeout < 1*time.Second {
		destroyTimeout = 1 * time.Second
	}

	select {
	case <-ctx.Done():
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return ctx.Err()
	case <-time.After(destroyTimeout):
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return ErrRPCTimeout
	case respMsg := <-respChan:
		var resp protocol.DestroyBoxResp
		if err := respMsg.UnpackPayload(&resp); err != nil {
			return fmt.Errorf("unpack destroy response: %w", err)
		}
		if !resp.Success {
			return errors.New(resp.Error)
		}
		return nil
	}
}
