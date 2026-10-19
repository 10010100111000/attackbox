package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
)

// JWTClaims represents user session claims signed by Ed25519.
type JWTClaims struct {
	Sub          int64  `json:"sub"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion int    `json:"ver"`
	Exp          int64  `json:"exp"`
}

// SignJWT generates a compact Ed25519 JWT (<base64_payload>.<base64_sig>).
func SignJWT(privKey ed25519.PrivateKey, claims JWTClaims) (string, error) {
	data, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(privKey, data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifyJWT verifies an Ed25519 JWT in memory without database queries.
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
		return nil, ErrInvalidToken
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

// HashPassword hashes plaintext passwords using bcrypt with standard cost.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies plaintext against the bcrypt hash in constant time.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// LoadOrGenerateKeys loads an Ed25519 keypair from hex or generates a fresh one.
func LoadOrGenerateKeys(hexPrivKey string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if hexPrivKey != "" {
		data, err := hex.DecodeString(strings.TrimSpace(hexPrivKey))
		if err == nil && len(data) == ed25519.PrivateKeySize {
			priv := ed25519.PrivateKey(data)
			pub := priv.Public().(ed25519.PublicKey)
			return pub, priv, nil
		}
	}

	// Generate fresh keypair
	pub, priv, err := ed25519.GenerateKey(nil)
	return pub, priv, err
}

// GenerateRandomCode generates an 8-character alphanumeric invite code.
func GenerateRandomCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}
