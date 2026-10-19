package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

func generateTestKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key pair: %v", err)
	}
	return pub, priv
}

func TestSignAndVerifyTicket_Success(t *testing.T) {
	pub, priv := generateTestKeyPair(t)

	claims := &TicketClaims{
		JTI: "test-jti-123456",
		Sub: 1001,
		Sbx: "sbx_test8f2a",
		Typ: "pty",
		Exp: time.Now().Add(60 * time.Second).Unix(),
		CIP: "127.0.0.1",
	}

	tokenStr, err := SignTicket(claims, priv)
	if err != nil {
		t.Fatalf("SignTicket failed: %v", err)
	}

	verifiedClaims, err := VerifyTicket(tokenStr, pub)
	if err != nil {
		t.Fatalf("VerifyTicket failed: %v", err)
	}

	if verifiedClaims.JTI != claims.JTI || verifiedClaims.Sub != claims.Sub ||
		verifiedClaims.Sbx != claims.Sbx || verifiedClaims.Typ != claims.Typ ||
		verifiedClaims.Exp != claims.Exp || verifiedClaims.CIP != claims.CIP {
		t.Errorf("claims mismatch: got %+v, want %+v", verifiedClaims, claims)
	}
}

func TestVerifyTicket_TamperedPayload(t *testing.T) {
	pub, priv := generateTestKeyPair(t)

	claims := &TicketClaims{
		JTI: "test-jti-tamper",
		Sub: 1001,
		Sbx: "sbx_normal",
		Typ: "pty",
		Exp: time.Now().Add(60 * time.Second).Unix(),
	}

	tokenStr, err := SignTicket(claims, priv)
	if err != nil {
		t.Fatalf("SignTicket failed: %v", err)
	}

	parts := strings.Split(tokenStr, ".")
	// 篡改 Payload 中的前两个字符
	tamperedPayload := "ab" + parts[0][2:]
	tamperedToken := tamperedPayload + "." + parts[1]

	_, err = VerifyTicket(tamperedToken, pub)
	if err == nil {
		t.Fatalf("expected signature verification failure for tampered payload, got nil")
	}
}

func TestVerifyTicket_Expired(t *testing.T) {
	pub, priv := generateTestKeyPair(t)

	claims := &TicketClaims{
		JTI: "test-jti-expired",
		Sub: 1001,
		Sbx: "sbx_expired",
		Typ: "vnc",
		Exp: time.Now().Add(-10 * time.Second).Unix(), // 已经过期 10 秒
	}

	tokenStr, err := SignTicket(claims, priv)
	if err != nil {
		t.Fatalf("SignTicket failed: %v", err)
	}

	_, err = VerifyTicket(tokenStr, pub)
	if err != ErrTicketExpired {
		t.Fatalf("expected ErrTicketExpired, got: %v", err)
	}
}

func TestVerifyTicket_InvalidStreamType(t *testing.T) {
	_, priv := generateTestKeyPair(t)

	claims := &TicketClaims{
		JTI: "test-jti-invalid-type",
		Sub: 1001,
		Sbx: "sbx_test",
		Typ: "ssh", // 不支持的流类型
		Exp: time.Now().Add(60 * time.Second).Unix(),
	}

	_, err := SignTicket(claims, priv)
	if err == nil {
		t.Fatalf("expected SignTicket to reject invalid stream type 'ssh', got nil")
	}
}

func TestJTIReplayFilter(t *testing.T) {
	filter := NewJTIReplayFilter(100 * time.Millisecond)
	defer filter.Close()

	jti := "jti-unique-token-001"
	ttl := 200 * time.Millisecond

	// 1. 首次消费应该成功
	if err := filter.CheckAndConsume(jti, ttl); err != nil {
		t.Fatalf("first consume should succeed, got: %v", err)
	}

	// 2. 立即重放必须拦截
	if err := filter.CheckAndConsume(jti, ttl); err != ErrReplayedTicket {
		t.Fatalf("expected ErrReplayedTicket for replay attack, got: %v", err)
	}

	// 3. 消费不同的 JTI 应该成功
	if err := filter.CheckAndConsume("jti-unique-token-002", ttl); err != nil {
		t.Fatalf("consume different jti should succeed, got: %v", err)
	}

	// 4. 等待 TTL 过期后，再次消费应允许 (或已被清理)
	time.Sleep(300 * time.Millisecond)
	if err := filter.CheckAndConsume(jti, ttl); err != nil {
		t.Fatalf("consume after TTL expiration should succeed, got: %v", err)
	}
}

func TestMessagePackUnpack(t *testing.T) {
	req := &CreateBoxReq{
		SandboxID: "sbx_test_01",
		UserID:    888,
		SlotIndex: 1,
		Policy: SandboxPolicy{
			Resources: BoxResourceQuota{
				CPUWeight:   100,
				MemoryMaxMB: 2048,
				PIDsMax:     512,
			},
			Storage: BoxStoragePolicy{
				TmpfsTmpSize: "512M",
			},
			Network: BoxNetworkPolicy{
				EgressMode: "restricted",
			},
		},
		Target: TargetSpec{
			Enabled: true,
			LabID:   "web-sqli-lab",
			Image:   "vuln-targets/sqli-app:v1",
			Runtime: "runc",
		},
	}

	msg, err := NewMessage(MsgTypeCreateBox, req)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Type != MsgTypeCreateBox {
		t.Errorf("msg.Type = %s, want %s", msg.Type, MsgTypeCreateBox)
	}

	var unpacked CreateBoxReq
	if err := msg.UnpackPayload(&unpacked); err != nil {
		t.Fatalf("UnpackPayload failed: %v", err)
	}

	if unpacked.SandboxID != req.SandboxID || unpacked.UserID != req.UserID ||
		unpacked.Policy.Resources.MemoryMaxMB != 2048 || unpacked.Target.Image != req.Target.Image {
		t.Errorf("unpacked mismatch: got %+v, want %+v", unpacked, req)
	}
}
