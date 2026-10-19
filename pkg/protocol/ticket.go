package protocol

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidTicketFormat = errors.New("invalid ticket format")
	ErrTicketExpired       = errors.New("ticket has expired")
	ErrInvalidSignature    = errors.New("invalid ticket signature")
	ErrReplayedTicket      = errors.New("ticket jti has already been consumed (replay attack)")
	ErrInvalidStreamType   = errors.New("unsupported stream type")
)

// TicketClaims 定义用户浏览器直连 Worker 数据信道 (:8443) 的能力凭证载荷
type TicketClaims struct {
	JTI string `json:"jti"`           // 凭据唯一 ID (UUID 或高熵 Hex，用于防重放)
	Sub int64  `json:"sub"`           // 所属用户 ID
	Sbx string `json:"sbx"`           // 绑定的沙箱唯一标识 (sbx_xxx)
	Typ string `json:"typ"`           // 授权数据流类型: "pty" (终端) 或 "vnc" (桌面)
	Exp int64  `json:"exp"`           // 过期时间戳 (秒，签发后固定 60 秒有效)
	CIP string `json:"cip,omitempty"` // 客户端公网 IP (可选校验)
}

// SignTicket 由 Control 中心调用，使用 Ed25519 私钥为沙箱访问签发 Ticket
// 输出格式: <base64url(payload)>.<base64url(signature)>
func SignTicket(claims *TicketClaims, privKey ed25519.PrivateKey) (string, error) {
	if claims == nil {
		return "", errors.New("claims cannot be nil")
	}
	if len(privKey) != ed25519.PrivateKeySize {
		return "", errors.New("invalid ed25519 private key size")
	}
	if claims.Typ != "pty" && claims.Typ != "vnc" {
		return "", fmt.Errorf("%w: %s", ErrInvalidStreamType, claims.Typ)
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal ticket claims: %w", err)
	}

	sig := ed25519.Sign(privKey, payloadBytes)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return encodedPayload + "." + encodedSig, nil
}

// VerifyTicket 由 Worker 算力节点调用，使用 Ed25519 公钥进行纯内存数学验签
// 耗时 < 0.1ms，零网络 I/O，零数据库查询
func VerifyTicket(tokenStr string, pubKey ed25519.PublicKey) (*TicketClaims, error) {
	if len(pubKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid ed25519 public key size")
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidTicketFormat
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid payload encoding", ErrInvalidTicketFormat)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature encoding", ErrInvalidTicketFormat)
	}

	// 1. Ed25519 非对称签名数学校验
	if !ed25519.Verify(pubKey, payloadBytes, sigBytes) {
		return nil, ErrInvalidSignature
	}

	// 2. 反序列化 Claims
	var claims TicketClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: unmarshal claims: %v", ErrInvalidTicketFormat, err)
	}

	// 3. 校验时效性 (固定 60s TTL)
	if time.Now().Unix() > claims.Exp {
		return nil, ErrTicketExpired
	}

	// 4. 校验流类型
	if claims.Typ != "pty" && claims.Typ != "vnc" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStreamType, claims.Typ)
	}

	return &claims, nil
}

// JTIReplayFilter Worker 本地纯内存 JTI 防重放拦截器
type JTIReplayFilter struct {
	mu      sync.RWMutex
	records map[string]time.Time // jti -> expiresAt
	stopCh  chan struct{}
}

// NewJTIReplayFilter 初始化防重放过滤器并启动后台周期清理协程
func NewJTIReplayFilter(cleanupInterval time.Duration) *JTIReplayFilter {
	f := &JTIReplayFilter{
		records: make(map[string]time.Time),
		stopCh:  make(chan struct{}),
	}
	go f.gcLoop(cleanupInterval)
	return f
}

// CheckAndConsume 尝试核销指定 JTI。
// 若 JTI 未使用过，记录并返回 nil；若已存在，返回 ErrReplayedTicket
func (f *JTIReplayFilter) CheckAndConsume(jti string, ttl time.Duration) error {
	if jti == "" {
		return errors.New("empty jti")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if expireAt, exists := f.records[jti]; exists {
		if time.Now().Before(expireAt) {
			return ErrReplayedTicket
		}
	}

	f.records[jti] = time.Now().Add(ttl)
	return nil
}

func (f *JTIReplayFilter) gcLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopCh:
			return
		case now := <-ticker.C:
			f.mu.Lock()
			for jti, expireAt := range f.records {
				if now.After(expireAt) {
					delete(f.records, jti)
				}
			}
			f.mu.Unlock()
		}
	}
}

// Close 停止后台清理协程
func (f *JTIReplayFilter) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}
}
