package control

import (
	"attackbox/protocol"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNoWorkersAvailable = errors.New("no active worker nodes available in cluster")
	ErrWorkerNotFound     = errors.New("worker node not found")
	ErrRPCTimeout         = errors.New("worker RPC request timed out")
)

// WorkerNode represents an active sacrificial worker connection in the cluster pool.
type WorkerNode struct {
	NodeID            string
	RemoteIP          string
	BootID            string
	LastSeq           uint64
	InventoryRevision uint64
	Ready             bool

	stateMu         sync.RWMutex
	State           protocol.NodeState
	DataPort        int
	PublicDataURL   string
	Capabilities    protocol.NodeCapabilities
	Telemetry       protocol.HeartbeatReq
	ActiveSandboxes int
	LastHeartbeat   time.Time

	conn       *websocket.Conn
	connMu     sync.Mutex
	pendingMu  sync.Mutex
	pendingRPC map[string]chan *protocol.Message
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
	w.Ready = false
	w.Capabilities = reg.Capabilities
	if reg.BootID != "" {
		w.BootID = reg.BootID
	}
	if reg.DataPort > 0 {
		w.DataPort = reg.DataPort
	}
	if reg.PublicDataURL != "" {
		w.PublicDataURL = reg.PublicDataURL
	}
}

func (w *WorkerNode) GetDataURL(fallbackHost string) string {
	return w.GetStreamURL("pty", fallbackHost)
}

func (w *WorkerNode) GetStreamURL(streamType, fallbackHost string) string {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	endpointPath := "/ws/cli"
	if streamType == "vnc" || streamType == "gui" {
		endpointPath = "/ws/gui"
	}
	if w.PublicDataURL != "" {
		return strings.TrimSuffix(w.PublicDataURL, "/") + endpointPath
	}
	host := w.RemoteIP
	if host == "" || host == "127.0.0.1" {
		host = fallbackHost
	}
	port := w.DataPort
	if port <= 0 {
		port = 8443
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return "ws://" + net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port)) + endpointPath
}

// Accept only monotonic inventories from the registered daemon generation.
func (w *WorkerNode) UpdateHeartbeat(hb protocol.HeartbeatReq) bool {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	if hb.BootID == "" || hb.BootID != w.BootID || hb.Seq <= w.LastSeq || hb.InventoryRevision < w.InventoryRevision {
		return false
	}
	w.LastSeq = hb.Seq
	w.InventoryRevision = hb.InventoryRevision
	w.LastHeartbeat = time.Now()
	w.Telemetry = hb
	w.ActiveSandboxes = hb.ActiveSandboxes
	return true
}

func (w *WorkerNode) Generation() string {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.BootID
}

func (w *WorkerNode) IsReady() bool { w.stateMu.RLock(); defer w.stateMu.RUnlock(); return w.Ready }

func (w *WorkerNode) GetSchedulingSnapshot() (state protocol.NodeState, lastHB time.Time, activeSandboxes int, dataPort int) {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.State, w.LastHeartbeat, w.ActiveSandboxes, w.DataPort
}

// NodeClusterManager manages active worker WebSocket connections and coordinates RPC dispatching.
type inventoryClock struct {
	BootID        string
	Seq, Revision uint64
}

type NodeClusterManager struct {
	clusterSecret       string
	pubKeyHex           string
	heartbeatTimeoutSec int
	rpcTimeoutSec       int
	onHeartbeat         func(workerID string, hb protocol.HeartbeatReq)
	mu                  sync.RWMutex
	workers             map[string]*WorkerNode
	clocks              map[string]inventoryClock
	desiredStates       map[string]protocol.NodeState
	upgrader            websocket.Upgrader
}

// SetHeartbeatHandler registers a callback invoked whenever a worker reports a heartbeat.
func (h *NodeClusterManager) SetHeartbeatHandler(fn func(workerID string, hb protocol.HeartbeatReq)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onHeartbeat = fn
}

// SetNodeState transitions a worker node's lifecycle state (Online, Draining, Offline, Resetting).
func (h *NodeClusterManager) SetNodeState(nodeID string, state protocol.NodeState) error {
	h.mu.Lock()
	w, exists := h.workers[nodeID]
	if exists {
		h.desiredStates[nodeID] = state
		w.SetState(state)
	}
	h.mu.Unlock()
	if !exists {
		return ErrWorkerNotFound
	}
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
	return h.GetWorkerStreamURL(nodeID, "pty", fallbackHost)
}

func (h *NodeClusterManager) GetWorkerStreamURL(nodeID, streamType, fallbackHost string) (string, bool) {
	h.mu.RLock()
	node, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return "", false
	}
	return node.GetStreamURL(streamType, fallbackHost), true
}

// NewNodeClusterManager creates a cluster manager.
func NewNodeClusterManager(clusterSecret string) *NodeClusterManager {
	return &NodeClusterManager{
		clusterSecret:       clusterSecret,
		heartbeatTimeoutSec: 15,
		rpcTimeoutSec:       30,
		workers:             make(map[string]*WorkerNode),
		clocks:              make(map[string]inventoryClock), desiredStates: make(map[string]protocol.NodeState),
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

func (h *NodeClusterManager) registerWorker(w *WorkerNode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// If a worker with the same nodeID was previously connected, close the stale connection
	// to prevent orphaned goroutines and avoid leaking old RPC loops.
	if old, exists := h.workers[w.NodeID]; exists && old != w && old.conn != nil {
		_ = old.conn.Close()
	}
	if desired, ok := h.desiredStates[w.NodeID]; ok {
		w.SetState(desired)
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
			continue
		}
		if state != protocol.NodeStateOnline || !w.IsReady() {
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

func (h *NodeClusterManager) SetInventoryReady(nodeID, bootID string, seq uint64, ready bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if w := h.workers[nodeID]; w != nil {
		w.stateMu.Lock()
		defer w.stateMu.Unlock()
		if w.BootID == bootID && w.Telemetry.Seq == seq {
			w.Ready = ready
		}
	}
}

func (h *NodeClusterManager) IsCurrentInventory(nodeID string, hb protocol.HeartbeatReq) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	w := h.workers[nodeID]
	if w == nil {
		return false
	}
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.BootID == hb.BootID && w.Telemetry.Seq == hb.Seq
}

func (h *NodeClusterManager) WorkerBootID(nodeID string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if w := h.workers[nodeID]; w != nil {
		return w.Generation()
	}
	return ""
}
