package control

import (
	"attackbox/protocol"
	"context"
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

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
	defer conn.Close()

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

func (h *NodeClusterManager) handleWorkerMessage(node *WorkerNode, msg *protocol.Message) {
	h.mu.RLock()
	current := h.workers[node.NodeID] == node
	h.mu.RUnlock()
	if !current {
		return
	}

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
			if reg.BootID == "" {
				_ = node.conn.Close()
				return
			}
			h.mu.Lock()
			if h.workers[node.NodeID] != node {
				h.mu.Unlock()
				return
			}
			previous := h.clocks[node.NodeID]
			node.UpdateRegister(reg)
			node.stateMu.Lock()
			if previous.BootID == reg.BootID {
				node.LastSeq = previous.Seq
				node.InventoryRevision = previous.Revision
			}
			node.stateMu.Unlock()
			h.mu.Unlock()
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
			accepted := node.UpdateHeartbeat(hb)
			if !accepted {
				return
			}

			h.mu.Lock()
			if h.workers[node.NodeID] != node {
				h.mu.Unlock()
				return
			}
			h.clocks[node.NodeID] = inventoryClock{BootID: hb.BootID, Seq: hb.Seq, Revision: hb.InventoryRevision}
			hbHandler := h.onHeartbeat
			h.mu.Unlock()
			if hbHandler != nil {
				hbHandler(node.NodeID, hb)
			}
			ack, _ := protocol.NewMessage(protocol.MsgTypeHeartbeatAck, protocol.HeartbeatResp{Success: true})
			_ = node.Send(ack)
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
