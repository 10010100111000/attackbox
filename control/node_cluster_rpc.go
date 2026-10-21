package control

import (
	"attackbox/protocol"
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"time"
)

func (w *WorkerNode) Send(msg *protocol.Message) error {
	data, err := protocol.PackMessage(msg)
	if err != nil {
		return err
	}
	w.connMu.Lock()
	defer w.connMu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

// CreateBox dispatches a CreateBox RPC with fine-grained policy to the target worker.
func (h *NodeClusterManager) CreateBox(ctx context.Context, nodeID string, req protocol.CreateBoxReq) (*protocol.CreateBoxResp, error) {
	h.mu.RLock()
	node, exists := h.workers[nodeID]
	h.mu.RUnlock()
	if !exists {
		return nil, ErrWorkerNotFound
	}
	if !node.IsReady() || req.BootID != node.Generation() {
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

	h.mu.RLock()
	rpcTimeout := time.Duration(h.rpcTimeoutSec) * time.Second
	h.mu.RUnlock()
	if rpcTimeout <= 0 {
		rpcTimeout = 30 * time.Second
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
			return &resp, errors.New(resp.Error)
		}
		if resp.SandboxID != req.SandboxID || resp.BootID != node.Generation() || resp.InventoryRevision == 0 || resp.ExpiresAt <= 0 {
			return nil, errors.New("invalid worker create acknowledgement")
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
	req.BootID = node.Generation()

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
	h.mu.RLock()
	destroyTimeout := time.Duration(h.rpcTimeoutSec) * time.Second
	h.mu.RUnlock()
	if destroyTimeout <= 0 {
		destroyTimeout = 15 * time.Second
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
		if resp.SandboxID != req.SandboxID || resp.BootID != req.BootID {
			return errors.New("invalid worker cleanup acknowledgement")
		}
		return nil
	}
}
