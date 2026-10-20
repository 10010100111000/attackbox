package control

import (
	"attackbox/protocol"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

type JWTClaims struct {
	Sub          int64  `json:"sub"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion int    `json:"ver"`
	Exp          int64  `json:"exp"`
}

func SignJWT(privKey ed25519.PrivateKey, claims JWTClaims) (string, error) {
	data, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(privKey, data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func VerifyJWT(pubKey ed25519.PublicKey, tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !ed25519.Verify(pubKey, payload, sig) {
		return nil, ErrInvalidSignature
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Precomputed valid bcrypt hash for constant-time comparison against non-existent users
const dummyBcryptHash = "$2a$10$wT5Y1eS6Fqm9g4O/rQhGceJzR0d8z1bS3h1J6D8W2Y1bS3h1J6D8W"

func (s *ControlHTTPServer) issueTicket(uid int64, sandboxID string) (string, error) {
	jti := generateJTI()
	claims := &protocol.TicketClaims{
		Sub: uid,
		Sbx: sandboxID,
		Typ: "pty",
		Exp: time.Now().Add(time.Duration(s.cfg.Auth.TicketTTLSec) * time.Second).Unix(),
		JTI: jti,
	}
	return protocol.SignTicket(claims, s.privKey)
}

func generateJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
