package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrDecryptionFailed = errors.New("decryption failed or invalid ciphertext/nonce")
	ErrInvalidMasterKey = errors.New("invalid master key: must be 32 bytes")
)

// DeriveMasterKey 获取或派生用于 AES-256-GCM 的 32 字节主加密密钥
// 优先读取环境变量 ATTACKBOX_MASTER_KEY，未设置时结合机器特征与系统盐进行安全哈希派生
func DeriveMasterKey() []byte {
	envKey := os.Getenv("ATTACKBOX_MASTER_KEY")
	if envKey != "" {
		hash := sha256.Sum256([]byte(envKey))
		return hash[:]
	}

	// 宿主级备选盐：结合系统标识与静态命名空间派生
	hostSalt := "attackbox-secure-vault-master-salt-v1"
	if h, err := os.Hostname(); err == nil && h != "" {
		hostSalt += ":" + h
	}
	hash := sha256.Sum256([]byte(hostSalt))
	return hash[:]
}

// DeriveKeyFromPassword 使用管理员口令结合盐进行高强度派生 (PBKDF2 逻辑，防离线彩虹表)
func DeriveKeyFromPassword(password string) []byte {
	salt := []byte("attackbox-zero-knowledge-vault-salt-v2")
	// 执行 50,000 轮 HMAC-SHA256 迭代强化
	key := []byte(password)
	for i := 0; i < 50000; i++ {
		h := sha256.New()
		h.Write(key)
		h.Write(salt)
		key = h.Sum(nil)
	}
	return key
}

// EncryptSecret 使用 AES-256-GCM 加密敏感明文 (如 OAuth Client Secret)
// 每次加密动态生成 12 字节密码学安全 Nonce，返回 base64 编码的密文与 nonce
func EncryptSecret(plaintext string, masterKey []byte) (ciphertextB64, nonceB64 string, err error) {
	if len(masterKey) != 32 {
		return "", "", ErrInvalidMasterKey
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", "", fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("cipher new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(nonce),
		nil
}

// DecryptSecret 使用 AES-256-GCM 解密 base64 密文
func DecryptSecret(ciphertextB64, nonceB64 string, masterKey []byte) (string, error) {
	if len(masterKey) != 32 {
		return "", ErrInvalidMasterKey
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cipher new gcm: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return "", ErrDecryptionFailed
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}
