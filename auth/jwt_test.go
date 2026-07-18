package auth

import (
	"bytes"
	"testing"
	"time"
)

func TestJWTsRoundTripAndExpiry(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	clock := now
	tokens, err := NewJWTs(JWTConfig{
		Issuer: "example", Audience: "example-api", Secret: bytes.Repeat([]byte{7}, 32),
		Lifetime: time.Minute, Now: func() time.Time { return clock },
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := tokens.Issue("user-1", []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tokens.Verify(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || len(claims.Roles) != 1 || claims.Roles[0] != "admin" {
		t.Fatalf("claims = %#v", claims)
	}
	clock = now.Add(2 * time.Minute)
	if _, err := tokens.Verify(encoded); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestJWTsRejectsWeakSecret(t *testing.T) {
	if _, err := NewJWTs(JWTConfig{Issuer: "example", Audience: "api", Secret: []byte("weak"), Lifetime: time.Hour}); err == nil {
		t.Fatal("weak secret was accepted")
	}
}
