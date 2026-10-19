package cluster

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"attackbox/pkg/protocol"
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
	DataPort        int
	Capabilities    protocol.NodeCapabilities
	Telemetry       protocol.HeartbeatReq
	ActiveSandboxes int
	LastHeartbeat   time.Time
	conn            *websocket.Conn
	connMu          sync.Mutex
	pendingMu       sync.Mutex
	pendingRPC      map[string]chan *protocol.Message
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

// Hub manages active worker WebSocket connections and coordinates RPC dispatching.
type Hub struct {
	clusterSecret string
	mu            sync.RWMutex
	workers       map[string]*WorkerNode
	upgrader      websocket.Upgrader
}

// NewHub creates a cluster manager.
func NewHub(clusterSecret string) *Hub {
	return &Hub{
		clusterSecret: clusterSecret,
		workers:       make(map[string]*WorkerNode),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1 << 15,
			WriteBufferSize: 1 << 15,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// HandleAgentConnect handles reverse WebSocket connection handshakes from Worker agents.
func (h *Hub) HandleAgentConnect(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate cluster_secret
	authHeader := r.Header.Get("Authorization")
	secret := strings.TrimPrefix(authHeader, "Bearer ")
	if secret == "" {
		secret = r.URL.Query().Get("secret")
	}

	if secret != h.clusterSecret {
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

	node := &WorkerNode{
		NodeID:        nodeID,
		RemoteIP:      remoteIP,
		DataPort:      8443,
		LastHeartbeat: time.Now(),
		conn:          conn,
		pendingRPC:    make(map[string]chan *protocol.Message),
	}

	h.registerWorker(node)
	defer h.unregisterWorker(nodeID)

	log.Printf("[CLUSTER] Worker %s connected from %s", nodeID, remoteIP)

	// Inbound message loop
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[CLUSTER] Worker %s disconnected: %v", nodeID, err)
			break
		}

		var msg protocol.Message
		if err := protocol.UnpackMessage(raw, &msg); err != nil {
			continue
		}

		h.handleWorkerMessage(node, &msg)
	}
}

func (h *Hub) registerWorker(w *WorkerNode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.workers[w.NodeID] = w
}

func (h *Hub) unregisterWorker(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.workers, nodeID)
}

func (h *Hub) handleWorkerMessage(node *WorkerNode, msg *protocol.Message) {
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
			node.Capabilities = reg.Capabilities
			if reg.DataPort > 0 {
				node.DataPort = reg.DataPort
			}
			// Send RegisterAck
			ack, _ := protocol.NewMessage(protocol.MsgTypeRegisterAck, protocol.RegisterResp{
				Success: true,
				NodeID:  node.NodeID,
			})
			_ = node.Send(ack)
			log.Printf("[CLUSTER] Registered Worker %s (CPUs: %d, DataPort: %d)", node.NodeID, node.Capabilities.CPUCores, node.DataPort)
		}

	case protocol.MsgTypeHeartbeat:
		var hb protocol.HeartbeatReq
		if err := msg.UnpackPayload(&hb); err == nil {
			node.LastHeartbeat = time.Now()
			node.Telemetry = hb
			node.ActiveSandboxes = hb.ActiveSandboxes
			// Send HeartbeatAck
			ack, _ := protocol.NewMessage(protocol.MsgTypeHeartbeatAck, protocol.HeartbeatResp{Success: true})
			_ = node.Send(ack)
		}
	}
}

// SelectBestWorker applies least-connections scheduling across active workers.
func (h *Hub) SelectBestWorker() (*WorkerNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.workers) == 0 {
		return nil, ErrNoWorkersAvailable
	}

	var best *WorkerNode
	minLoad := int(^uint(0) >> 1) // max int

	for _, w := range h.workers {
		// Filter out nodes that haven't sent heartbeats in 15s
		if time.Since(w.LastHeartbeat) > 15*time.Second {
			continue
		}
		if w.ActiveSandboxes < minLoad {
			minLoad = w.ActiveSandboxes
			best = w
		}
	}

	if best == nil {
		return nil, ErrNoWorkersAvailable
	}
	return best, nil
}

// CreateBox dispatches a CreateBox RPC with fine-grained policy to the target worker.
func (h *Hub) CreateBox(ctx context.Context, nodeID string, req protocol.CreateBoxReq) (*protocol.CreateBoxResp, error) {
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

	select {
	case <-ctx.Done():
		node.pendingMu.Lock()
		delete(node.pendingRPC, msg.ID)
		node.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
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

// DestroyBox dispatches a DestroyBox RPC to the worker.
func (h *Hub) DestroyBox(ctx context.Context, nodeID string, req protocol.DestroyBoxReq) error {
	h.mu.RLock()
	node, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return nil // If worker already disconnected, nothing to destroy
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeDestroyBox, req)
	if err != nil {
		return err
	}

	return node.Send(msg)
}
