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
