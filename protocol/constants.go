package protocol

import "time"

// Network & Gateway Defaults
const (
	DefaultDataPort    = 8443
	DefaultControlPort = 18080
	DefaultGatewayPort = 18443
	DefaultMaxSlots    = 250
	DefaultBridgeName  = "br-attackbox"
	DefaultGatewayIP   = "10.200.0.1"
	DefaultSubnetCIDR  = "10.200.0.0/16"
)

// Timeout & Interval Defaults
const (
	DefaultHeartbeatInterval = 3 * time.Second
	DefaultHeartbeatTimeout  = 15 * time.Second
	DefaultRPCTimeout        = 30 * time.Second
	DefaultDestroyTimeout    = 15 * time.Second
	DefaultInitTimeout       = 10 * time.Second
	DefaultTicketTTLSec      = 60
	DefaultJWTExpireHours    = 24
	DefaultBackoffInitial    = 1 * time.Second
	DefaultBackoffMax        = 15 * time.Second
)

// Resource & Security Limits
const (
	MaxHTTPPayloadSize      = 1 << 20       // 1MB request body limit
	MaxWebSocketMessageSize = 1 << 20       // 1MB control message limit
	MaxPTYFrameSize         = 64 * 1024     // 64KB PTY interactive stream chunk
	DefaultMemLimitMB       = 2048
	DefaultPIDsLimit        = 512
	DefaultCPUWeight        = 100
	DefaultIOWriteBytesSec  = 20971520      // 20MB/s
	DefaultIOWriteIOPS      = 500
	DefaultTmpfsTmpSize     = "512M"
	DefaultTmpfsRunSize     = "256M"
	DefaultTmpfsDevShmSize  = "256M"
)
