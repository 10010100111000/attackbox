package auth

import (
	"testing"
)

func TestCrypto_EncryptDecrypt(t *testing.T) {
	key := DeriveMasterKey()
	if len(key) != 32 {
		t.Fatalf("expected 32-byte master key, got %d", len(key))
	}

	secret := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"

	ct1, nonce1, err := EncryptSecret(secret, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	ct2, nonce2, err := EncryptSecret(secret, key)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	// 验证随机 Nonce 与密文独立性 (即使明文相同，密文也绝不相同)
	if nonce1 == nonce2 {
		t.Fatalf("expected distinct random nonces, got duplicate")
	}
	if ct1 == ct2 {
		t.Fatalf("expected distinct ciphertexts, got duplicate")
	}

	// 验证解密
	pt, err := DecryptSecret(ct1, nonce1, key)
	if err != nil {
		t.Fatalf("decrypt ct1 failed: %v", err)
	}
	if pt != secret {
		t.Fatalf("expected %s, got %s", secret, pt)
	}

	// 验证篡改密文导致解密失败 (GCM 认证标签校验)
	tamperedCt := "A" + ct1[1:]
	_, err = DecryptSecret(tamperedCt, nonce1, key)
	if err == nil {
		t.Fatalf("expected error on tampered ciphertext, got nil")
	}
}
