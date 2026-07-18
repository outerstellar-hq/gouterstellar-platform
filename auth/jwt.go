package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	minJWTLifetime = time.Second
	maxJWTLifetime = 15 * time.Minute
	maxJWTLeeway   = time.Minute
)

// JWTConfig defines a deliberately narrow HMAC token profile. Issuer,
// audience, expiry and signing method are mandatory and verified on parse.
type JWTConfig struct {
	Issuer   string
	Audience string
	Secret   []byte
	Lifetime time.Duration
	Leeway   time.Duration
	Now      func() time.Time
}

// Claims is the application-neutral identity carried by a platform JWT.
type Claims struct {
	Roles []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// JWTs issues and verifies short-lived bearer tokens with one enforced profile.
type JWTs struct {
	issuer   string
	audience string
	secret   []byte
	lifetime time.Duration
	leeway   time.Duration
	now      func() time.Time
}

// NewJWTs validates configuration. A 256-bit secret is the minimum for HS256.
func NewJWTs(config JWTConfig) (*JWTs, error) {
	if config.Issuer == "" || config.Audience == "" {
		return nil, errors.New("JWT issuer and audience are required")
	}
	if len(config.Secret) < 32 {
		return nil, errors.New("JWT secret must contain at least 32 bytes")
	}
	if config.Lifetime < minJWTLifetime || config.Lifetime > maxJWTLifetime {
		return nil, fmt.Errorf("JWT lifetime must be between %s and %s", minJWTLifetime, maxJWTLifetime)
	}
	if config.Leeway < 0 || config.Leeway > maxJWTLeeway {
		return nil, fmt.Errorf("JWT leeway must be between zero and %s", maxJWTLeeway)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &JWTs{
		issuer: config.Issuer, audience: config.Audience,
		secret: append([]byte(nil), config.Secret...), lifetime: config.Lifetime,
		leeway: config.Leeway, now: config.Now,
	}, nil
}

// Issue creates a signed token for subject and copies roles into its claims.
func (j *JWTs) Issue(subject string, roles []string) (string, error) {
	if subject == "" {
		return "", errors.New("JWT subject is required")
	}
	now := j.now().UTC()
	claims := Claims{
		Roles: append([]string(nil), roles...),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{j.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.lifetime)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return token, nil
}

// Verify parses a token using a fixed algorithm allow-list and mandatory
// issuer, audience and expiration checks.
func (j *JWTs) Verify(encoded string) (Claims, error) {
	claims := Claims{}
	token, err := jwt.ParseWithClaims(
		encoded, &claims, func(*jwt.Token) (any, error) {
			return j.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(j.issuer),
		jwt.WithAudience(j.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(j.leeway),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("verify JWT: %w", err)
	}
	if !token.Valid || claims.Subject == "" {
		return Claims{}, errors.New("verify JWT: invalid claims")
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return Claims{}, errors.New("verify JWT: issued-at and expiration are required")
	}
	lifetime := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	if lifetime <= 0 || lifetime > j.lifetime {
		return Claims{}, errors.New("verify JWT: token lifetime exceeds configured profile")
	}
	claims.Roles = append([]string(nil), claims.Roles...)
	return claims, nil
}
