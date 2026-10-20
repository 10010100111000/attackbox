package control

import (
	"crypto/ed25519"
	"encoding/hex"
	"strings"
)

func LoadOrGenerateKeys(hexPrivKey string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if hexPrivKey != "" {
		data, err := hex.DecodeString(strings.TrimSpace(hexPrivKey))
		if err == nil && len(data) == ed25519.PrivateKeySize {
			priv := ed25519.PrivateKey(data)
			pub := priv.Public().(ed25519.PublicKey)
			return pub, priv, nil
		}
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	return pub, priv, err
}
