package auth

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewTokenSeparatesPlaintextAndDigest(t *testing.T) {
	token, err := NewToken("example_")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token.Plaintext, "example_") || len(token.Digest) != 32 {
		t.Fatalf("unexpected token: %#v", token)
	}
	if !bytes.Equal(token.Digest, TokenDigest(token.Plaintext)) {
		t.Fatal("digest does not match plaintext")
	}
}

func TestNewTokenRejectsUnsafeOrUnboundedPrefix(t *testing.T) {
	for name, prefix := range map[string]string{
		"slash":     "example/",
		"space":     "example token",
		"control":   "example\n",
		"non-ASCII": "tök_",
		"too long":  strings.Repeat("x", maxTokenPrefixBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewToken(prefix); err == nil {
				t.Fatal("expected token prefix validation error")
			}
		})
	}
}

func TestTokenHasherIsKeyed(t *testing.T) {
	first, err := NewTokenHasher(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewTokenHasher(bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Digest("token"), second.Digest("token")) {
		t.Fatal("different peppers produced the same digest")
	}
}
