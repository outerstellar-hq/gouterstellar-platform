package security

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
)

type AuthResult interface{ authResult() }

type AuthenticatedResult struct{ User *model.User }

func (AuthenticatedResult) authResult() {}

type ExpiredResult struct{}

func (ExpiredResult) authResult() {}

type SkippedResult struct{}

func (SkippedResult) authResult() {}

type AuthRealm interface {
	Name() string
	Authenticate(token string) AuthResult
}

type (
	SessionLookupFunc func(token string) model.SessionLookup
	ApiKeyLookupFunc  func(rawKey string) *model.User
	JwtLookupFunc     func(userID uuid.UUID) *model.User
)

type sessionRealm struct {
	lookup SessionLookupFunc
}

func NewSessionRealm(lookup SessionLookupFunc) AuthRealm {
	return &sessionRealm{lookup: lookup}
}

func (r *sessionRealm) Name() string { return "session" }

func (r *sessionRealm) Authenticate(token string) AuthResult {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	result := r.lookup(tokenHash)

	switch v := result.(type) {
	case model.SessionActive:
		return AuthenticatedResult{User: v.User}
	case model.SessionExpired:
		return ExpiredResult{}
	default:
		return SkippedResult{}
	}
}

type apiKeyRealm struct {
	lookup ApiKeyLookupFunc
}

func NewApiKeyRealm(lookup ApiKeyLookupFunc) AuthRealm {
	return &apiKeyRealm{lookup: lookup}
}

func (r *apiKeyRealm) Name() string { return "apikey" }

func (r *apiKeyRealm) Authenticate(rawKey string) AuthResult {
	user := r.lookup(rawKey)
	if user != nil {
		return AuthenticatedResult{User: user}
	}
	return SkippedResult{}
}

type jwtRealm struct {
	jwtSvc *JwtService
	lookup JwtLookupFunc
}

func NewJwtRealm(jwtSvc *JwtService, lookup JwtLookupFunc) AuthRealm {
	return &jwtRealm{jwtSvc: jwtSvc, lookup: lookup}
}

func (r *jwtRealm) Name() string { return "jwt" }

func (r *jwtRealm) Authenticate(token string) AuthResult {
	if !r.jwtSvc.IsEnabled() {
		return SkippedResult{}
	}

	userID, _, err := r.jwtSvc.ExtractClaims(token)
	if err != nil {
		return SkippedResult{}
	}

	user := r.lookup(userID)
	if user != nil {
		return AuthenticatedResult{User: user}
	}
	return SkippedResult{}
}
