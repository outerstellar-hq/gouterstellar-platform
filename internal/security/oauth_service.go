package security

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

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
		pltUser, err := s.userRepo.FindByID(ctx, conn.UserID)
		if err != nil {
			return nil, fmt.Errorf("OAuth user not found in users table: %w", err)
		}
		return PltUserToModel(pltUser), nil
	}

	slog.Info("No existing OAuth connection, creating new user", "provider", providerName, "subject", oauthSubject)

	username := deriveUsernameFromEmail(email)
	userEmail := ""
	if email != nil {
		userEmail = *email
	}

	randomPassword := generateRandomPassword()
	passwordHash, err := s.encoder.Encode(randomPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to encode random password: %w", err)
	}

	userID := uuid.New()
	pltUser, err := s.userRepo.CreateUser(ctx, userID, username, userEmail, passwordHash, string(model.RoleUser), true)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth user: %w", err)
	}

	_, err = s.oauthRepo.SaveOAuthConnection(ctx, userID, providerName, oauthSubject, email)
	if err != nil {
		return nil, fmt.Errorf("failed to save OAuth connection: %w", err)
	}

	return PltUserToModel(pltUser), nil
}

func deriveUsernameFromEmail(email *string) string {
	if email != nil && *email != "" {
		parts := strings.SplitN(*email, "@", 2)
		if parts[0] != "" {
			return parts[0]
		}
	}
	return "oauth_user_" + uuid.New().String()[:8]
}

func generateRandomPassword() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
