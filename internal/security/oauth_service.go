package security

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
)

type OAuthService struct {
	userRepo  persistence.UserRepository
	oauthRepo persistence.OAuthRepository
	encoder   PasswordEncoder
}

func NewOAuthService(userRepo persistence.UserRepository, oauthRepo persistence.OAuthRepository, encoder PasswordEncoder) *OAuthService {
	return &OAuthService{
		userRepo:  userRepo,
		oauthRepo: oauthRepo,
		encoder:   encoder,
	}
}

func (s *OAuthService) FindOrCreateOAuthUser(ctx context.Context, providerName, oauthSubject string, email *string) (*model.User, error) {
	conn, err := s.oauthRepo.FindByProviderSubject(ctx, providerName, oauthSubject)
	if err == nil {
		return s.findConnectedUser(ctx, conn.UserID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find OAuth connection: %w", err)
	}

	baseUsername := deriveOAuthUsername(providerName, email)
	username, err := s.ensureUniqueUsername(ctx, baseUsername)
	if err != nil {
		return nil, err
	}
	userEmail := username + "@" + providerName + ".oauth"
	if email != nil {
		userEmail = *email
	}

	randomPassword := generateRandomPassword()
	passwordHash, err := s.encoder.Encode(randomPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to encode random password: %w", err)
	}

	userID := uuid.New()
	pltUser, err := s.oauthRepo.CreateUserAndConnection(
		ctx, userID, username, userEmail, passwordHash, providerName, oauthSubject, email,
	)
	if err != nil {
		winner, findErr := s.oauthRepo.FindByProviderSubject(ctx, providerName, oauthSubject)
		if findErr == nil {
			return s.findConnectedUser(ctx, winner.UserID)
		}
		return nil, fmt.Errorf("create OAuth identity: %w", err)
	}

	return PltUserToModel(pltUser), nil
}

func (s *OAuthService) findConnectedUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	pltUser, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("OAuth user not found in users table: %w", err)
	}
	return PltUserToModel(pltUser), nil
}

func (s *OAuthService) ensureUniqueUsername(ctx context.Context, base string) (string, error) {
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s%d", base, suffix)
		}
		_, err := s.userRepo.FindByUsername(ctx, candidate)
		if errors.Is(err, pgx.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("check OAuth username availability: %w", err)
		}
	}
}

func deriveOAuthUsername(providerName string, email *string) string {
	if email != nil && *email != "" {
		parts := strings.SplitN(*email, "@", 2)
		var filtered strings.Builder
		for _, r := range parts[0] {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				filtered.WriteRune(r)
			}
			if filtered.Len() >= 30 {
				break
			}
		}
		if filtered.Len() > 0 {
			return filtered.String()
		}
	}
	return providerName + "_" + uuid.New().String()[:8]
}

func generateRandomPassword() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
