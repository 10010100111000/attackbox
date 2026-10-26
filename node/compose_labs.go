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
type composeLabs struct {
	runtime      *labcompose.Manager
	network      *labnetwork.Manager
	preparations *composePreparations
	commands     chan struct{}
	stops        chan struct{}
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
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
	labs := &composeLabs{runtime: runtime, network: network, ctx: ctx, cancel: cancel, commands: make(chan struct{}, 8), stops: make(chan struct{}, 4)}
	labs.preparations = newComposePreparations(ctx, func(ctx context.Context, req protocol.ComposeRequest) (labcompose.Deployment, error) {
		dir, err := downloadComposeBundle(ctx, cfg, req.BundleID, req.BundleHash)
		if err != nil {
			return labcompose.Deployment{}, err
		}
		defer os.RemoveAll(dir)
		s := req.Settings
		return runtime.Prepare(ctx, labcompose.Bundle{Directory: dir, File: s.File, Variables: req.Variables, EntryService: s.EntryService, EntryNetwork: s.EntryNetwork, EntryPort: s.EntryPort, PrefixBits: s.PrefixBits, Rebuild: req.Rebuild})
	}, runtime.Deployment)
	return labs, nil
}
func (l *composeLabs) Close() {
	l.cancel()
	l.wg.Wait()
	l.preparations.Close()
	l.runtime.Close()
	l.network.Close()
}
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
	case "Docker Buildx plugin required for local builds":
		return "节点缺少 Docker Buildx 插件，请安装 docker-buildx-plugin 后重试准备"
	case "local image build failed":
		return "本地镜像构建失败，请检查 Dockerfile、构建参数、基础镜像及节点构建环境"
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
	if msg.Type == protocol.MsgComposeStop || msg.Type == protocol.MsgComposePrepareCancel {
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
	if msg.Type == protocol.MsgComposePrepare || msg.Type == protocol.MsgComposePrepareStatus || msg.Type == protocol.MsgComposePrepareCancel {
		job := c.compose.preparations.handle(req, msg.Type)
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
		Rebuild     bool
		Settings    protocol.ComposeSettings
		Variables   map[string]string
	}{req.OwnerID, req.BundleHash, req.Rebuild, req.Settings, req.Variables})
	return bundleDigest(raw)
}
