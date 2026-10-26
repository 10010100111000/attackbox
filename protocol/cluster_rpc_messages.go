package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
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

// NodeCapabilities 节点硬件与功能真实探测字典
type NodeCapabilities struct {
	HasKVM     bool   `json:"has_kvm"`      // 是否支持 /dev/kvm
	CPUCores   int    `json:"cpu_cores"`    // 物理 CPU 核心数
	TotalMemMB int64  `json:"total_mem_mb"` // 物理总内存 (MB, 真实读取 /proc/meminfo)
	OSVersion  string `json:"os_version"`   // 宿主机系统版本
}

// RegisterReq 节点向控制端主动注册请求
type RegisterReq struct {
	NodeID           string           `json:"node_id"`
	BootID           string           `json:"boot_id,omitempty"` // 节点启动唯一标识
	Version          string           `json:"version"`
	DataPort         int              `json:"data_port"`                    // 数据通道监听端口 (默认 8443)
	PublicDataURL    string           `json:"public_data_url,omitempty"`    // 节点对外数据服务基准 URL (如 wss://...)
	Capabilities     NodeCapabilities `json:"capabilities"`                 // 硬件与运行时能力
	ActiveSandboxIDs []string         `json:"active_sandbox_ids,omitempty"` // 本地活跃沙箱 ID 清单
}

// RegisterResp 控制端注册响应
type RegisterResp struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	NodeID    string `json:"node_id"`
	PublicKey string `json:"public_key,omitempty"` // Hex-encoded Ed25519 Public Key
}

// NodeState 节点全生命周期状态机
type NodeState string

const (
	NodeStateOnline    NodeState = "Online"    // 正常心跳在线，处于调度池
	NodeStateDraining  NodeState = "Draining"  // 节点处于维护排空期，仅运行存量，不接受新分配
	NodeStateOffline   NodeState = "Offline"   // 心跳超时断线，自动剔除出调度队列
	NodeStateResetting NodeState = "Resetting" // 正在执行底层云快照回滚重建
)

// SandboxSummary 节点心跳中上报的极简沙箱概要
type SandboxSummary struct {
	State        string `json:"state"`
	ID           string `json:"id"`
	LastActiveAt int64  `json:"last_active_at"` // Unix timestamp (秒)
	GUIRunning   bool   `json:"gui_running,omitempty"`
}

// HeartbeatReq 节点周期心跳真实遥测 (每 3 秒)
type HeartbeatReq struct {
	ComposeLabs       *ComposeCapability `json:"compose_labs,omitempty"`
	InventoryRevision uint64             `json:"inventory_revision"`
	NodeID            string             `json:"node_id"`
	BootID            string             `json:"boot_id,omitempty"` // 节点启动唯一标识
	Seq               uint64             `json:"seq,omitempty"`     // 节点递增序列号
	Timestamp         int64              `json:"timestamp"`
	LoadAvg           float64            `json:"load_avg"`                     // 宿主机 1分钟平均负载 (真实读取 /proc/loadavg)
	MemAvailableMB    int64              `json:"mem_available_mb"`             // 真实可用物理内存 MB (真实读取 /proc/meminfo)
	DiskFreeGB        int64              `json:"disk_free_gb"`                 // 真实磁盘剩余 GB
	ActiveSandboxes   int                `json:"active_sandboxes"`             // 真实活跃沙箱运行数
	ActiveSandboxIDs  []string           `json:"active_sandbox_ids,omitempty"` // 本地活跃沙箱 ID 清单
	Sandboxes         []SandboxSummary   `json:"sandboxes,omitempty"`          // 极简快照清单
}

// HeartbeatResp 控制端心跳响应
type HeartbeatResp struct {
	Success bool `json:"success"`
}

// BoxResourceQuota 精细化物理资源配额 (Cgroups v2)
type BoxResourceQuota struct {
	CPUWeight       uint64 `json:"cpu_weight" yaml:"cpu_weight"`                 // Cgroups v2 cpu.weight (默认 100)
	MemoryMaxMB     int64  `json:"memory_max_mb" yaml:"memory_max_mb"`           // 物理内存硬限额 (MB)
	MemorySwapMaxMB int64  `json:"memory_swap_max_mb" yaml:"memory_swap_max_mb"` // Swap 内存 (MB, 0 为完全禁用)
	PIDsMax         int64  `json:"pids_max" yaml:"pids_max"`                     // 最大并发进程数 (默认 256)
	IOWriteBytesSec int64  `json:"io_write_bytes_sec" yaml:"io_write_bytes_sec"` // 磁盘写入带宽限制 (字节/秒)
	IOWriteIOPS     int64  `json:"io_write_iops" yaml:"io_write_iops"`           // 磁盘写入 IOPS 限制
}

// BoxStoragePolicy 精细化存储与挂载策略
type BoxStoragePolicy struct {
	TmpfsTmpSize string `json:"tmpfs_tmp_size" yaml:"tmpfs_tmp_size"` // /tmp 内存盘大小 (如 "512M")
	TmpfsRunSize string `json:"tmpfs_run_size" yaml:"tmpfs_run_size"` // /run 内存盘大小 (如 "256M")
}

// UnmarshalYAML supports both tmpfs_tmp_size/tmp_size and tmpfs_run_size/run_size keys from YAML.
func (s *BoxStoragePolicy) UnmarshalYAML(value *yaml.Node) error {
	var m map[string]string
	if err := value.Decode(&m); err != nil {
		return err
	}
	for key := range m {
		switch key {
		case "tmpfs_tmp_size", "tmp_size", "tmpfs_run_size", "run_size":
		default:
			return fmt.Errorf("unknown storage setting %q", key)
		}
	}
	if a, b := m["tmpfs_tmp_size"], m["tmp_size"]; a != "" && b != "" && a != b {
		return fmt.Errorf("conflicting tmpfs_tmp_size and tmp_size")
	}
	if a, b := m["tmpfs_run_size"], m["run_size"]; a != "" && b != "" && a != b {
		return fmt.Errorf("conflicting tmpfs_run_size and run_size")
	}
	if v, ok := m["tmpfs_tmp_size"]; ok {
		s.TmpfsTmpSize = v
	} else if v, ok := m["tmp_size"]; ok {
		s.TmpfsTmpSize = v
	}
	if v, ok := m["tmpfs_run_size"]; ok {
		s.TmpfsRunSize = v
	} else if v, ok := m["run_size"]; ok {
		s.TmpfsRunSize = v
	}
	return nil
}

// BoxSecurityPolicy 精细化沙箱安全策略 (只保留内核真正消费的策略)
type BoxSecurityPolicy struct {
	ReadOnlyProcSys bool `json:"read_only_proc_sys" yaml:"read_only_proc_sys"`
	MaskSysrq       bool `json:"mask_sysrq" yaml:"mask_sysrq"`
}

// SandboxPolicy 控制端向节点下发的综合精细化策略包（全管道单一 JSON 透传）
type SandboxPolicy struct {
	Resources BoxResourceQuota  `json:"resources"`
	Storage   BoxStoragePolicy  `json:"storage"`
	Security  BoxSecurityPolicy `json:"security"`
}

// CreateBoxReq 控制端下发沙箱创建指令
type CreateBoxReq struct {
	OwnerID     string        `json:"owner_id,omitempty"`
	BootID      string        `json:"boot_id"`
	TTLSeconds  int           `json:"ttl_seconds"`
	IdleSeconds int           `json:"idle_seconds"`
	SandboxID   string        `json:"sandbox_id"`
	UserID      int64         `json:"user_id"`
	SlotIndex   int           `json:"slot_index"` // 网络槽位索引 (1-250)
	Policy      SandboxPolicy `json:"policy"`     // 完整细粒度策略
}

// CreateBoxResp 节点响应沙箱创建结果
type CreateBoxResp struct {
	InventoryRevision uint64 `json:"inventory_revision"`
	ExpiresAt         int64  `json:"expires_at"`
	BootID            string `json:"boot_id"`
	SandboxID         string `json:"sandbox_id"`
	Success           bool   `json:"success"`
	Error             string `json:"error,omitempty"`
	SandboxIP         string `json:"sandbox_ip,omitempty"`
}

// DestroyBoxReq 控制端下发沙箱销毁指令
type DestroyBoxReq struct {
	BootID    string `json:"boot_id"`
	SandboxID string `json:"sandbox_id"`
	Reason    string `json:"reason"`
}

// DestroyBoxResp 节点响应沙箱销毁结果
type DestroyBoxResp struct {
	BootID    string `json:"boot_id"`
	SandboxID string `json:"sandbox_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}
