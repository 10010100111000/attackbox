package node

import (
	"attackbox/protocol"
	"context"
	"time"
)

func (c *NodeControlClient) targetCommandLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case msg := <-c.targetQueue:
			if ctx.Err() != nil {
				return
			}
			opCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
			resp := c.executeTargetCommand(opCtx, msg)
			cancel()
			c.sendTargetResponse(msg, resp)
		}
	}
}

func (c *NodeControlClient) executeTargetCommand(ctx context.Context, msg *protocol.Message) protocol.TargetResponse {
	var req protocol.TargetRequest
	resp := protocol.TargetResponse{BootID: c.bootID}
	if len(msg.Payload) == 0 || msg.UnpackPayload(&req) != nil {
		resp.Error = "invalid_target_request"
		return resp
	}
	resp.TargetID = req.TargetID
	if req.BootID != c.bootID {
		resp.Error = "worker_generation_changed"
		return resp
	}
	if c.targets == nil {
		resp.Error = "docker_targets_disabled"
		if c.targetInitError != "" {
			resp.Error = c.targetInitError
		}
		return resp
	}
	var instance protocol.TargetSummary
	var err error
	switch msg.Type {
	case protocol.MsgTypeTargetStart:
		if c.rtMgr != nil && c.rtMgr.DiskGuard().IsTripped() {
			resp.Error = "worker_disk_pressure"
			return resp
		}
		instance, err = c.targets.Start(ctx, req)
	case protocol.MsgTypeTargetInspect:
		instance, err = c.targets.Inspect(ctx, req)
	case protocol.MsgTypeTargetStop:
		instance, err = c.targets.Stop(ctx, req)
	default:
		resp.Error = "unknown_target_command"
		return resp
	}
	if instance.TargetID != "" {
		resp.Instance = &instance
	}
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	resp.Success = true
	return resp
}

func (c *NodeControlClient) sendTargetResponse(msg *protocol.Message, resp protocol.TargetResponse) {
	resp.BootID = c.bootID
	if resp.TargetID == "" {
		var req protocol.TargetRequest
		_ = msg.UnpackPayload(&req)
		resp.TargetID = req.TargetID
	}
	out, err := protocol.NewMessage(msg.Type+"_resp", resp)
	if err == nil {
		out.ID = msg.ID
		_ = c.sendMessage(out)
	}
}
