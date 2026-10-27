package protocol_test

import (
	"crypto/ed25519"
	"testing"
	"time"

	"attackbox/protocol"
)

func TestTicketSignAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	claims := &protocol.TicketClaims{
		JTI: "jti_test_abc",
		Sub: 1001,
		Sbx: "sbx_test_123",
		Typ: "pty",
		Exp: time.Now().Add(60 * time.Second).Unix(),
	}

	signed, err := protocol.SignTicket(claims, priv)
	if err != nil {
		t.Fatalf("SignTicket: %v", err)
	}

	verified, err := protocol.VerifyTicket(signed, pub)
	if err != nil {
		t.Fatalf("VerifyTicket: %v", err)
	}

	if verified.Sbx != "sbx_test_123" || verified.Sub != 1001 || verified.Typ != "pty" {
		t.Fatalf("Claims mismatch: %+v", verified)
	}
}

func TestJTIReplayFilter(t *testing.T) {
	filter := protocol.NewJTIReplayFilter(100 * time.Millisecond)
	defer filter.Close()

	jti := "nonce_test_1"
	if err := filter.CheckAndConsume(jti, 1*time.Second); err != nil {
		t.Fatalf("First consumption should succeed, got: %v", err)
	}

	if err := filter.CheckAndConsume(jti, 1*time.Second); err != protocol.ErrReplayedTicket {
		t.Fatalf("Second consumption should fail with ErrReplayedTicket, got: %v", err)
	}
}
