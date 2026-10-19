package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// 核心消息类型枚举
const (
	MsgTypeRegister       = "register"
	MsgTypeRegisterAck    = "register_ack"
	MsgTypeHeartbeat      = "heartbeat"
	MsgTypeHeartbeatAck   = "heartbeat_ack"
	MsgTypeCreateBox      = "create_box"
	MsgTypeCreateBoxResp  = "create_box_resp"
	MsgTypeDestroyBox     = "destroy_box"
	MsgTypeDestroyBoxResp = "destroy_box_resp"
	MsgTypePolicyUpdate   = "policy_update"
	MsgTypePolicyUpdateAck= "policy_update_ack"
	MsgTypeReconcile      = "reconcile"      // 断线重连沙箱状态对齐
	MsgTypeReconcileResp  = "reconcile_resp"
)

// Message 统一控制信道 JSON 消息封包帧
type Message struct {
	ID      string          `json:"id"`                // 消息唯一标识 (请求响应追踪)
	Type    string          `json:"type"`              // 消息类型
	Payload json.RawMessage `json:"payload,omitempty"` // 业务载荷
}

// NewMessage 构造新的通用消息封包
func NewMessage(msgType string, payload any) (*Message, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	msgID := hex.EncodeToString(b)

	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal message payload: %w", err)
		}
		raw = data
	}

	return &Message{
		ID:      msgID,
		Type:    msgType,
		Payload: raw,
	}, nil
}

// UnpackPayload 便捷解包当前 Message 的载荷到指定结构体
func (m *Message) UnpackPayload(target any) error {
	if len(m.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(m.Payload, target)
}

// ----------------- 载荷结构定义 -----------------

// NodeCapabilities 节点硬件与功能探测字典
type NodeCapabilities struct {
	HasOverlayFS bool   `json:"has_overlayfs"` // 是否支持 OverlayFS
	HasKVM       bool   `json:"has_kvm"`       // 是否支持 /dev/kvm (硬件虚拟化逃逸能力)
	HasDocker    bool   `json:"has_docker"`    // 是否运行 Docker 守护进程
	CPUCores     int    `json:"cpu_cores"`     // 物理 CPU 核心数
	TotalMemMB   int64  `json:"total_mem_mb"`  // 物理总内存 (MB)
	OSVersion    string `json:"os_version"`    // 宿主机系统版本
}

// RegisterReq Worker 主动注册请求
type RegisterReq struct {
	NodeID       string           `json:"node_id"`
	Version      string           `json:"version"`
	DataPort     int              `json:"data_port"`    // 数据通道监听端口 (默认 8443)
	Capabilities NodeCapabilities `json:"capabilities"` // 硬件与运行时能力
}

// RegisterResp Control 注册响应
type RegisterResp struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	NodeID  string `json:"node_id"`
}

// HeartbeatReq Worker 周期心跳遥测 (每 3 秒)
type HeartbeatReq struct {
	NodeID          string  `json:"node_id"`
	Timestamp       int64   `json:"timestamp"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"` // 0.0 - 100.0
	MemFreeMB       int64   `json:"mem_free_mb"`
	DiskFreeGB      int64   `json:"disk_free_gb"`
	ActiveSandboxes int     `json:"active_sandboxes"` // 当前正在运行的沙箱数
}

// HeartbeatResp Control 心跳响应
type HeartbeatResp struct {
	Success bool `json:"success"`
}

// BoxResourceQuota 沙箱物理配额与限制参数
type BoxResourceQuota struct {
	CPUWeight   int   `json:"cpu_weight"`   // Cgroups v2 cpu.weight (默认 100)
	MemoryMaxMB int64 `json:"memory_max_mb"` // 物理内存硬限额 (默认 2048 MB)
	PIDsMax     int   `json:"pids_max"`     // 最大并发进程数防 Fork 炸弹 (默认 512)
	DiskLimitMB int64 `json:"disk_limit_mb"`// upperdir Project Quota 限额 (默认 10240 MB)
}

// TargetSpec 靶标容器规格
type TargetSpec struct {
	Enabled      bool              `json:"enabled"`       // 是否伴生启动靶标容器
	LabID        string            `json:"lab_id"`        // 靶场模板标识
	Image        string            `json:"image"`         // 镜像名称
	Runtime      string            `json:"runtime"`       // "runc" (默认) 或 "kata"
	ExposedPorts []int             `json:"exposed_ports"` // 需向沙箱暴露的端口
	Env          map[string]string `json:"env"`           // 环境变量 (含动态注入 Flag)
}

// CreateBoxReq Control 指派 Worker 创建沙箱
type CreateBoxReq struct {
	SandboxID string           `json:"sandbox_id"` // 全局唯一沙箱 ID (sbx_xxx)
	UserID    int64            `json:"user_id"`    // 所属用户 ID
	Quotas    BoxResourceQuota `json:"quotas"`     // 资源配额约束
	Target    TargetSpec       `json:"target"`     // 靶机容器规格
}

// CreateBoxResp Worker 沙箱创建结果
type CreateBoxResp struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SandboxID string `json:"sandbox_id"`
	SandboxIP string `json:"sandbox_ip"` // 沙箱内部 IP (如 10.200.0.5)
	TargetIP  string `json:"target_ip"`  // 靶机容器动态 IPAM IP (如 172.17.0.8)
}

// DestroyBoxReq Control 销毁沙箱指令
type DestroyBoxReq struct {
	SandboxID string `json:"sandbox_id"`
	Reason    string `json:"reason"` // "user_logout", "timeout", "points_exhausted", "admin_kill"
}

// DestroyBoxResp Worker 销毁沙箱结果
type DestroyBoxResp struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SandboxID string `json:"sandbox_id"`
}

// PolicyUpdateReq 动态安全策略与 Feature Flags 热下发 (免重启 Worker)
type PolicyUpdateReq struct {
	AllowPtrace   bool `json:"allow_ptrace"`   // 是否放行 ptrace 调试能力
	DefaultMemory int64`json:"default_memory"` // 默认内存限额
	MaxPIDs       int  `json:"max_pids"`       // 默认 PIDs 上限
}

// ReconcileReq 断线重连后 Worker 上报当前本地活跃的沙箱列表
type ReconcileReq struct {
	NodeID          string   `json:"node_id"`
	ActiveSandboxIDs[]string `json:"active_sandbox_ids"`
}

// ReconcileResp Control 下发状态对齐指令，列出需要清理的孤儿沙箱
type ReconcileResp struct {
	OrphanSandboxIDs []string `json:"orphan_sandbox_ids"` // 本地有多余、DB已不存在的孤儿沙箱
}
