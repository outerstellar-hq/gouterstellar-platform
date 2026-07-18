package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

const (
	defaultTokenBytes   = 32
	maxTokenPrefixBytes = 64
)

// TokenPair separates a bearer credential from the digest safe to persist.
type TokenPair struct {
	Plaintext string
	Digest    []byte
}

// NewToken creates an opaque, URL-safe bearer token. Only Digest should be
// stored; Plaintext should be returned to the caller exactly once.
func NewToken(prefix string) (TokenPair, error) {
	if !validTokenPrefix(prefix) {
		return TokenPair{}, fmt.Errorf("token prefix must contain at most %d RFC 3986 unreserved ASCII characters", maxTokenPrefixBytes)
	}
	random := make([]byte, defaultTokenBytes)
	if _, err := rand.Read(random); err != nil {
		return TokenPair{}, fmt.Errorf("generate token: %w", err)
	}
	plaintext := prefix + base64.RawURLEncoding.EncodeToString(random)
	return TokenPair{Plaintext: plaintext, Digest: TokenDigest(plaintext)}, nil
}

func validTokenPrefix(prefix string) bool {
	if len(prefix) > maxTokenPrefixBytes {
		return false
	}
	for i := range len(prefix) {
		char := prefix[i]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '.' || char == '_' || char == '~' {
			continue
		}
		return false
	}
	return true
}

// TokenDigest returns the SHA-256 digest used for exact token lookup.
func TokenDigest(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

// TokenHasher produces deterministic keyed digests when a database lookup
// requires defense in depth beyond a high-entropy opaque token.
type TokenHasher struct {
	key []byte
}

// NewTokenHasher copies a pepper so callers may safely clear their source
// buffer after construction.
func NewTokenHasher(pepper []byte) (*TokenHasher, error) {
	if len(pepper) < 32 {
		return nil, errors.New("token pepper must contain at least 32 bytes")
	}
	return &TokenHasher{key: append([]byte(nil), pepper...)}, nil
}

// Digest returns an HMAC-SHA-256 digest suitable for indexed storage.
func (h *TokenHasher) Digest(token string) []byte {
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte(token))
	return mac.Sum(nil)
}
