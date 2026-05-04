package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"

	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/model"
)

type JwtService struct {
	cfg   config.JwtConfig
	cache *cache.Cache
}

func NewJwtService(cfg config.JwtConfig) *JwtService {
	return &JwtService{
		cfg:   cfg,
		cache: cache.New(60*time.Second, 120*time.Second),
	}
}

func (s *JwtService) IsEnabled() bool {
	return s.cfg.Enabled && s.cfg.Secret != ""
}

type jwtClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
}

func (s *JwtService) GenerateToken(user *model.User) (string, error) {
	if !s.IsEnabled() {
		return "", errors.New("JWT is not enabled")
	}

	now := time.Now().UTC()
	expiry := now.Add(time.Duration(s.cfg.ExpirySeconds) * time.Second)

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    s.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
		Username: user.Username,
		Admin:    user.Role == model.RoleAdmin,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	s.cache.Set(signed, claims, cache.DefaultExpiration)
	return signed, nil
}

func (s *JwtService) ExtractClaims(tokenStr string) (userID uuid.UUID, isAdmin bool, err error) {
	if cached, found := s.cache.Get(tokenStr); found {
		if c, ok := cached.(jwtClaims); ok {
			uid, parseErr := uuid.Parse(c.Subject)
			if parseErr != nil {
				return uuid.Nil, false, fmt.Errorf("invalid subject in cached claims: %w", parseErr)
			}
			return uid, c.Admin, nil
		}
	}

	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return uuid.Nil, false, errors.New("invalid JWT claims")
	}

	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("invalid subject in JWT claims: %w", err)
	}

	s.cache.Set(tokenStr, *claims, cache.DefaultExpiration)
	return uid, claims.Admin, nil
}

func (s *JwtService) Invalidate(token string) {
	s.cache.Delete(token)
}
