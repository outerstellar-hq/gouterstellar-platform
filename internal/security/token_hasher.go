package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// TokenHasher produces deterministic, keyed digests for opaque tokens that
// must be looked up without storing the bearer credential itself.
type TokenHasher struct {
	key []byte
}

func NewTokenHasher(pepper string) TokenHasher {
	return TokenHasher{key: []byte(pepper)}
}

func (h TokenHasher) Hash(token string) string {
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte(token))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
