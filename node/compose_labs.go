package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"attackbox/node/labcompose"
	"attackbox/node/labnetwork"
	rt "attackbox/node/runtime"
	"attackbox/protocol"
)

type ComposeConfig struct {
	Enabled         bool   `yaml:"enabled"`
	NetworkVerified bool   `yaml:"network_verified"`
	StateDir        string `yaml:"state_dir"`
	SocketPath      string `yaml:"socket_path"`
	Pool            string `yaml:"pool"`
}
type composeJob struct {
	Logs         []string
	State        string
	DeploymentID string
	Summary      *protocol.ComposeDefinition
	Error        string
	Fingerprint  string
}
type composeLabs struct {
	runtime  *labcompose.Manager
	network  *labnetwork.Manager
	mu       sync.Mutex
	jobs     map[string]composeJob
	prepare  chan struct{}
	commands chan struct{}
	stops    chan struct{}
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func newComposeLabs(cfg *NodeConfig, sandboxes *rt.RuntimeManager) (*composeLabs, error) {
	if !cfg.ComposeLabs.NetworkVerified {
		return nil, fmt.Errorf("compose_network_not_verified")
	}
	c := &cfg.ComposeLabs
	if c.StateDir == "" {
		c.StateDir = "/var/lib/attackbox/compose-labs"
	}
	if c.SocketPath == "" {
		c.SocketPath = "/var/run/docker.sock"
	}
	if c.Pool == "" {
		c.Pool = "10.202.0.0/16"
	}
	pool, err := netip.ParsePrefix(c.Pool)
	if err != nil {
		return nil, fmt.Errorf("invalid Compose network pool")
	}
	excluded := []netip.Prefix{}
	for _, s := range []string{cfg.Network.SubnetCIDR, func() string {
		if cfg.Network.DockerTargets.Enabled {
			return cfg.Network.DockerTargets.SubnetCIDR
		}
		return ""
	}()} {
		if s != "" {
			p, err := netip.ParsePrefix(s)
			if err != nil {
				return nil, err
			}
			excluded = append(excluded, p)
		}
	}
	network, err := labnetwork.New(labnetwork.Config{NodeID: cfg.Node.NodeID, StateDir: filepath.Join(c.StateDir, "network"), DockerSocket: c.SocketPath, Pool: pool, Excluded: excluded})
	if err != nil {
		return nil, err
	}
	engine, err := labcompose.NewCLI(labcompose.CLIConfig{DockerSocket: c.SocketPath})
	if err != nil {
		network.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtimeConfig := labcompose.Config{NodeID: cfg.Node.NodeID, StateDir: filepath.Join(c.StateDir, "runtime"), Engine: engine, Networks: network}
	if sandboxes != nil {
		runtimeConfig.SandboxAccess = sandboxes.WithLabSandbox
	}
	runtime, err := labcompose.New(ctx, runtimeConfig)
	if err != nil {
		cancel()
		network.Close()
		return nil, err
	}
	return &composeLabs{runtime: runtime, network: network, ctx: ctx, cancel: cancel, jobs: map[string]composeJob{}, prepare: make(chan struct{}, 1), commands: make(chan struct{}, 8), stops: make(chan struct{}, 4)}, nil
}
func (l *composeLabs) Close() { l.cancel(); l.wg.Wait(); l.runtime.Close(); l.network.Close() }
func definitionSummary(d labcompose.Deployment) *protocol.ComposeDefinition {
	s := &protocol.ComposeDefinition{Networks: d.Definition.Networks, EntryService: d.Definition.EntryService, EntryNetwork: d.Definition.EntryNetwork, EntryPort: d.Definition.EntryPort, Report: d.Definition.Report, Services: []protocol.ComposeServiceDefinition{}}
	services, _ := d.Definition.Document["services"].(map[string]any)
	for name, v := range services {
		m, _ := v.(map[string]any)
		image, _ := m["image"].(string)
		s.Services = append(s.Services, protocol.ComposeServiceDefinition{Name: name, Image: image})
	}
	sort.Slice(s.Services, func(i, j int) bool { return s.Services[i].Name < s.Services[j].Name })
	return s
}
func (l *composeLabs) job(req protocol.ComposeRequest, prepare bool, cfg *NodeConfig) composeJob {
	l.mu.Lock()
	defer l.mu.Unlock()
	old, exists := l.jobs[req.PreparationID]
	if !prepare {
		if exists {
			return old
		}
		if req.DeploymentID != "" {
			if d, err := l.runtime.Deployment(l.ctx, req.DeploymentID); err == nil {
				return composeJob{State: "ready", DeploymentID: d.ID, Summary: definitionSummary(d)}
			}
		}
		return composeJob{State: "unknown"}
	}
	if !bundleIDPattern.MatchString(req.PreparationID) || req.TargetID != req.PreparationID {
		return composeJob{State: "failed", Error: "invalid_preparation_identity"}
	}
	fingerprint := composeRequestFingerprint(req)
	if exists && old.Fingerprint != fingerprint {
		return composeJob{State: "failed", Error: "preparation_identity_changed"}
	}
	if exists && (old.State == "preparing" || old.State == "ready") {
		return old
	}
	if !exists && len(l.jobs) >= 256 {
		for id, job := range l.jobs {
			if job.State != "preparing" {
				delete(l.jobs, id)
				break
			}
		}
	}
	select {
	case l.prepare <- struct{}{}:
	default:
		return composeJob{State: "failed", Error: "compose_prepare_busy"}
	}
	l.jobs[req.PreparationID] = composeJob{State: "preparing", Fingerprint: fingerprint, Logs: []string{"已接收准备任务", "正在下载与校验部署包"}}
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		defer func() { <-l.prepare }()
		ctx, cancel := context.WithTimeout(l.ctx, 30*time.Minute)
		defer cancel()
		result := composeJob{State: "failed", Error: "compose_prepare_failed", Fingerprint: fingerprint, Logs: []string{"准备失败，请检查配置支持范围、配套文件与镜像访问权限"}}
		dir, err := downloadComposeBundle(ctx, cfg, req.BundleID, req.BundleHash)
		if err == nil {
			l.mu.Lock()
			progress := l.jobs[req.PreparationID]
			progress.Logs = append(progress.Logs, "部署包校验完成", "正在解析 Compose 与准备镜像")
			l.jobs[req.PreparationID] = progress
			l.mu.Unlock()
			defer os.RemoveAll(dir)
			s := req.Settings
			var d labcompose.Deployment
			d, err = l.runtime.Prepare(ctx, labcompose.Bundle{Directory: dir, File: s.File, Variables: req.Variables, EntryService: s.EntryService, EntryNetwork: s.EntryNetwork, EntryPort: s.EntryPort, PrefixBits: s.PrefixBits})
			if err == nil {
				result = composeJob{State: "ready", DeploymentID: d.ID, Summary: definitionSummary(d), Fingerprint: fingerprint, Logs: []string{"部署包校验完成", "Compose 校验与镜像准备完成", "已保存不可变部署产物"}}
			}
		}
		l.mu.Lock()
		if err != nil {
			result.Logs = append(append([]string(nil), l.jobs[req.PreparationID].Logs...), composeDiagnostic(err))
		}
		l.jobs[req.PreparationID] = result
		l.mu.Unlock()
	}()
	return l.jobs[req.PreparationID]
}

func composeDiagnostic(err error) string {
	message := err.Error()
	const prefix = "unsupported Compose field: "
	if strings.HasPrefix(message, prefix) {
		field := strings.TrimPrefix(message, prefix)
		if regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`).MatchString(field) {
			return "暂不支持 Compose 字段：" + field
		}
	}
	switch message {
	case "Docker operation failed":
		return "Docker 操作失败，请检查镜像引用、Registry 登录和节点网络"
	case "bundle download rejected":
		return "部署包下载未获授权，请检查控制端与 Worker 的集群配置"
	case "bundle digest mismatch":
		return "部署包摘要不匹配，请重新上传并准备"
	case "Compose validation failed; check fields, dependencies and required variables":
		return "Compose 校验失败，请检查字段、服务依赖和必填变量"
	case "package file missing":
		return "缺少部署所需的配套文件"
	}
	return "准备未完成，请检查节点可用性及当前支持的 Compose 配置范围"
}
func (c *NodeControlClient) dispatchCompose(msg *protocol.Message) {
	if c.compose == nil {
		c.sendCompose(msg, protocol.ComposeResponse{Error: "compose_unavailable"})
		return
	}
	select {
	case <-c.stopCh:
		c.sendCompose(msg, protocol.ComposeResponse{Error: "worker_stopping"})
		return
	default:
	}
	slots := c.compose.commands
	if msg.Type == protocol.MsgComposeStop {
		slots = c.compose.stops
	}
	select {
	case slots <- struct{}{}:
	default:
		c.sendCompose(msg, protocol.ComposeResponse{Error: "compose_queue_full"})
		return
	}
	c.compose.wg.Add(1)
	go func() {
		defer c.compose.wg.Done()
		defer func() { <-slots }()
		ctx, cancel := context.WithTimeout(c.compose.ctx, 125*time.Second)
		defer cancel()
		c.sendCompose(msg, c.executeCompose(ctx, msg))
	}()
}
func (c *NodeControlClient) sendCompose(msg *protocol.Message, resp protocol.ComposeResponse) {
	var req protocol.ComposeRequest
	_ = msg.UnpackPayload(&req)
	resp.BootID = c.bootID
	resp.TargetID = req.TargetID
	resp.OwnerID = req.OwnerID
	out, err := protocol.NewMessage(msg.Type+"_resp", resp)
	if err == nil {
		out.ID = msg.ID
		_ = c.sendMessage(out)
	}
}
func (c *NodeControlClient) executeCompose(ctx context.Context, msg *protocol.Message) protocol.ComposeResponse {
	var req protocol.ComposeRequest
	resp := protocol.ComposeResponse{}
	if msg.UnpackPayload(&req) != nil || req.BootID != c.bootID || !targetOwnerPattern.MatchString(req.OwnerID) {
		resp.Error = "invalid_compose_request"
		return resp
	}
	if c.compose == nil {
		resp.Error = "compose_unavailable"
		return resp
	}
	if (msg.Type == protocol.MsgComposePrepare || msg.Type == protocol.MsgComposeStart) && c.rtMgr != nil && c.rtMgr.DiskGuard().IsTripped() {
		resp.Error = "worker_disk_pressure"
		return resp
	}
	if msg.Type == protocol.MsgComposePrepare || msg.Type == protocol.MsgComposePrepareStatus {
		job := c.compose.job(req, msg.Type == protocol.MsgComposePrepare, c.cfg)
		resp.Success = true
		resp.JobState = job.State
		resp.DeploymentID = job.DeploymentID
		resp.Summary = job.Summary
		resp.Error = job.Error
		resp.Logs = job.Logs
		return resp
	}
	if !targetIDPattern.MatchString(req.TargetID) {
		resp.Error = "invalid_target_identity"
		return resp
	}
	if msg.Type == protocol.MsgComposeInspect && !c.compose.runtime.Owns(req.TargetID, req.OwnerID) {
		resp.Error = "compose_instance_not_found"
		return resp
	}
	var status labcompose.Status
	var err error
	switch msg.Type {
	case protocol.MsgComposeStart:
		status, err = c.compose.runtime.Start(ctx, labcompose.Request{ID: req.TargetID, OwnerID: req.OwnerID, DeploymentID: req.DeploymentID, ExpiresAt: req.ExpiresAt})
	case protocol.MsgComposeInspect:
		if c.rtMgr != nil {
			_ = c.rtMgr.WithLabSandbox(ctx, req.OwnerID, c.compose.runtime.SandboxAvailable)
		}
		status, err = c.compose.runtime.Inspect(ctx, req.TargetID)
	case protocol.MsgComposeStop:
		if !c.compose.runtime.Owns(req.TargetID, req.OwnerID) {
			report, reconcileErr := c.compose.network.Reconcile(ctx)
			if reconcileErr != nil || len(report.Orphans) > 0 {
				resp.Error = "compose_cleanup_unconfirmed"
				return resp
			}
		}
		status, err = c.compose.runtime.StopRequest(ctx, labcompose.Request{ID: req.TargetID, OwnerID: req.OwnerID, DeploymentID: req.DeploymentID, ExpiresAt: req.ExpiresAt})
	default:
		resp.Error = "unknown_compose_operation"
		return resp
	}
	if status.ID != "" {
		if status.Readiness == "" {
			status.Readiness = "unverified"
		}
		instance := &protocol.ComposeInstance{TargetID: status.ID, OwnerID: req.OwnerID, State: status.State, Ready: status.Ready, Readiness: status.Readiness, EntryAddress: status.EntryAddress, Attachment: status.Attachment, ExpiresAt: status.ExpiresAt, Services: []protocol.ComposeService{}}
		instance.AttackerAddress = status.AttackerAddress
		instance.Interface = status.Interface
		for _, s := range status.Services {
			instance.Services = append(instance.Services, protocol.ComposeService{Name: s.Name, State: s.State, Health: s.Health, IPs: s.IPs})
		}
		resp.Instance = instance
	}
	resp.Success = err == nil
	if err != nil {
		resp.Error = "compose_operation_unconfirmed"
	}
	return resp
}

// Package UUID is provenance, while content hash determines preparation identity.
func composeRequestFingerprint(req protocol.ComposeRequest) string {
	raw, _ := json.Marshal(struct {
		Owner, Hash string
		Settings    protocol.ComposeSettings
		Variables   map[string]string
	}{req.OwnerID, req.BundleHash, req.Settings, req.Variables})
	return bundleDigest(raw)
}
