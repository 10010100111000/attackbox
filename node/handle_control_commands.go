package node

import (
	rt "attackbox/node/runtime"
	"attackbox/protocol"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

type sandboxMailbox struct {
	ch chan *protocol.Message
}

func (c *NodeControlClient) DispatchMessageForTest(msg *protocol.Message) {
	c.dispatchMessage(msg)
}

func (c *NodeControlClient) dispatchMessage(msg *protocol.Message) {
	switch msg.Type {
	case protocol.MsgComposePrepare, protocol.MsgComposePrepareStatus, protocol.MsgComposePrepareCancel, protocol.MsgComposeStart, protocol.MsgComposeInspect, protocol.MsgComposeStop:
		c.dispatchCompose(msg)
	case protocol.MsgTypeTargetImagePrepare, protocol.MsgTypeTargetImageStatus, protocol.MsgTypeTargetStart, protocol.MsgTypeTargetInspect, protocol.MsgTypeTargetStop:
		select {
		case <-c.stopCh:
			c.sendTargetResponse(msg, protocol.TargetResponse{Error: "worker_stopping"})
		default:
			select {
			case c.targetQueue <- msg:
			default:
				c.sendTargetResponse(msg, protocol.TargetResponse{Error: "target_queue_full"})
			}
		}

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
		OwnerID:    req.OwnerID,
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
