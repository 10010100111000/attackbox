package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// 核心消息类型枚举
const (
	MsgTypeRegister        = "register"
	MsgTypeRegisterAck     = "register_ack"
	MsgTypeHeartbeat       = "heartbeat"
	MsgTypeHeartbeatAck    = "heartbeat_ack"
	MsgTypeCreateBox       = "create_box"
	MsgTypeCreateBoxResp   = "create_box_resp"
	MsgTypeDestroyBox      = "destroy_box"
	MsgTypeDestroyBoxResp  = "destroy_box_resp"
	MsgTypePolicyUpdate    = "policy_update"
	MsgTypePolicyUpdateAck = "policy_update_ack"
	MsgTypeReconcile       = "reconcile"
	MsgTypeReconcileResp   = "reconcile_resp"
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

// PackMessage 将 Message 结构体序列化为 JSON 字节
func PackMessage(m *Message) ([]byte, error) {
	return json.Marshal(m)
}

// UnpackMessage 将原始 JSON 字节反序列化为 Message 结构体
func UnpackMessage(data []byte, m *Message) error {
	return json.Unmarshal(data, m)
}

// ----------------- 载荷结构定义 -----------------

// NodeCapabilities 节点硬件与功能探测字典
type NodeCapabilities struct {
	HasOverlayFS bool   `json:"has_overlayfs"` // 是否支持 OverlayFS
	HasKVM       bool   `json:"has_kvm"`       // 是否支持 /dev/kvm
	HasDocker    bool   `json:"has_docker"`    // 是否运行 Docker 守护进程
	CPUCores     int    `json:"cpu_cores"`     // 物理 CPU 核心数
	TotalMemMB   int64  `json:"total_mem_mb"`  // 物理总内存 (MB)
	OSVersion    string `json:"os_version"`    // 宿主机系统版本
}

// RegisterReq 节点向控制端主动注册请求
type RegisterReq struct {
	NodeID       string           `json:"node_id"`
	Version      string           `json:"version"`
	DataPort     int              `json:"data_port"`    // 数据通道监听端口 (默认 8443)
	Capabilities NodeCapabilities `json:"capabilities"` // 硬件与运行时能力
}

// RegisterResp 控制端注册响应
type RegisterResp struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	NodeID    string `json:"node_id"`
	PublicKey string `json:"public_key,omitempty"` // Hex-encoded Ed25519 Public Key
}

// HeartbeatReq 节点周期心跳遥测 (每 3 秒)
type HeartbeatReq struct {
	NodeID          string  `json:"node_id"`
	Timestamp       int64   `json:"timestamp"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemFreeMB       int64   `json:"mem_free_mb"`
	DiskFreeGB      int64   `json:"disk_free_gb"`
	ActiveSandboxes int     `json:"active_sandboxes"`
}

// HeartbeatResp 控制端心跳响应
type HeartbeatResp struct {
	Success bool `json:"success"`
}

// BoxResourceQuota 精细化物理资源配额 (Cgroups v2)
type BoxResourceQuota struct {
	CPUWeight       uint64 `json:"cpu_weight"`         // Cgroups v2 cpu.weight (默认 100)
	MemoryMaxMB     int64  `json:"memory_max_mb"`      // 物理内存硬限额 (MB)
	MemorySwapMaxMB int64  `json:"memory_swap_max_mb"` // Swap 内存 (MB, 0 为完全禁用)
	PIDsMax         int64  `json:"pids_max"`           // 最大并发进程数 (默认 256)
	IOWriteBytesSec int64  `json:"io_write_bytes_sec"` // 磁盘写入带宽限制 (字节/秒)
	IOWriteIOPS     int64  `json:"io_write_iops"`      // 磁盘写入 IOPS 限制
}

// BoxStoragePolicy 精细化存储与挂载策略
type BoxStoragePolicy struct {
	TmpfsTmpSize string `json:"tmpfs_tmp_size"` // /tmp 内存盘大小 (如 "512M")
	TmpfsRunSize string `json:"tmpfs_run_size"` // /run 内存盘大小 (如 "256M")
}

// BoxNetworkPolicy 精细化网络管控策略
type BoxNetworkPolicy struct {
	EgressMode        string `json:"egress_mode"`         // "airgap" | "restricted" | "full"
	ExtraAllowedPorts []int  `json:"extra_allowed_ports"` // 额外放行出向端口
}

// BoxSecurityPolicy 精细化沙箱安全与环境策略
type BoxSecurityPolicy struct {
	ReadOnlyProcSys bool              `json:"read_only_proc_sys" yaml:"read_only_proc_sys"`
	MaskSysrq       bool              `json:"mask_sysrq" yaml:"mask_sysrq"`
	AllowPTrace     bool              `json:"allow_ptrace" yaml:"allow_ptrace"`
	ExtraEnv        map[string]string `json:"extra_env" yaml:"extra_env"`
}

// SandboxPolicy 控制端向节点下发的综合精细化策略包（全管道单一 JSON 透传）
type SandboxPolicy struct {
	Resources BoxResourceQuota  `json:"resources"`
	Storage   BoxStoragePolicy  `json:"storage"`
	Network   BoxNetworkPolicy  `json:"network"`
	Security  BoxSecurityPolicy `json:"security"`
}

// TargetSpec 靶机目标规格定义
type TargetSpec struct {
	Enabled      bool              `json:"enabled"`
	LabID        string            `json:"lab_id"`
	Image        string            `json:"image"`
	Runtime      string            `json:"runtime"`
	ExposedPorts []int             `json:"exposed_ports"`
	Env          map[string]string `json:"env"`
}

// CreateBoxReq 控制端下发沙箱创建指令
type CreateBoxReq struct {
	SandboxID string        `json:"sandbox_id"`
	UserID    int64         `json:"user_id"`
	SlotIndex int           `json:"slot_index"` // 网络槽位索引 (1-250)
	Policy    SandboxPolicy `json:"policy"`     // 完整细粒度策略
	Target    TargetSpec    `json:"target,omitempty"`
}

// CreateBoxResp 节点响应沙箱创建结果
type CreateBoxResp struct {
	SandboxID string `json:"sandbox_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SandboxIP string `json:"sandbox_ip,omitempty"`
	TargetIP  string `json:"target_ip,omitempty"`
}

// DestroyBoxReq 控制端下发沙箱销毁指令
type DestroyBoxReq struct {
	SandboxID string `json:"sandbox_id"`
	Reason    string `json:"reason"`
}

// DestroyBoxResp 节点响应沙箱销毁结果
type DestroyBoxResp struct {
	SandboxID string `json:"sandbox_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}
