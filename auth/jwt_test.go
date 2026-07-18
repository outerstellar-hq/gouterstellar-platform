package auth

import (
	"bytes"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func TestJWTsRejectsUnsafeTimeProfile(t *testing.T) {
	base := JWTConfig{
		Issuer: "example", Audience: "api", Secret: bytes.Repeat([]byte{7}, 32), Lifetime: time.Minute,
	}
	tests := map[string]JWTConfig{
		"subsecond lifetime": func() JWTConfig {
			config := base
			config.Lifetime = minJWTLifetime - time.Nanosecond
			return config
		}(),
		"excessive lifetime": func() JWTConfig {
			config := base
			config.Lifetime = maxJWTLifetime + time.Second
			return config
		}(),
		"negative leeway": func() JWTConfig {
			config := base
			config.Leeway = -time.Second
			return config
		}(),
		"excessive leeway": func() JWTConfig {
			config := base
			config.Leeway = maxJWTLeeway + time.Second
			return config
		}(),
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewJWTs(config); err == nil {
				t.Fatal("expected JWT time-profile error")
			}
		})
	}
	base.Lifetime = maxJWTLifetime
	base.Leeway = maxJWTLeeway
	if _, err := NewJWTs(base); err != nil {
		t.Fatalf("exact time-profile limits rejected: %v", err)
	}
}

func TestJWTsRejectsMissingIssuedAtAndLongerLifetime(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	secret := bytes.Repeat([]byte{7}, 32)
	tokens, err := NewJWTs(JWTConfig{
		Issuer: "example", Audience: "api", Secret: secret, Lifetime: 5 * time.Minute,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	sign := func(claims Claims) string {
		encoded, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	base := jwt.RegisteredClaims{
		Issuer: "example", Subject: "user-1", Audience: jwt.ClaimStrings{"api"},
		NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	}
	if _, err := tokens.Verify(sign(Claims{RegisteredClaims: base})); err == nil {
		t.Fatal("token without issued-at was accepted")
	}

	base.IssuedAt = jwt.NewNumericDate(now)
	base.ExpiresAt = jwt.NewNumericDate(now.Add(10 * time.Minute))
	if _, err := tokens.Verify(sign(Claims{RegisteredClaims: base})); err == nil {
		t.Fatal("token exceeding configured lifetime was accepted")
	}
}
