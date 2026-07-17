package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/security"
)

type PasswordResetService struct {
	userRepo        persistence.UserRepository
	passwordEncoder security.PasswordEncoder
	resetRepo       persistence.PasswordResetRepository
	emailService    EmailService
	auditor         Auditor
	appBaseURL      string
	tokenHasher     security.TokenHasher
}

func NewPasswordResetService(
	userRepo persistence.UserRepository,
	passwordEncoder security.PasswordEncoder,
	resetRepo persistence.PasswordResetRepository,
	emailService EmailService,
	auditor Auditor,
	appBaseURL string,
	tokenPepper string,
) *PasswordResetService {
	return &PasswordResetService{
		userRepo:        userRepo,
		passwordEncoder: passwordEncoder,
		resetRepo:       resetRepo,
		emailService:    emailService,
		auditor:         auditor,
		appBaseURL:      appBaseURL,
		tokenHasher:     security.NewTokenHasher(tokenPepper),
	}
}

func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email string) (*string, error) {
	pltUser, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("Password reset requested for unknown email")
			return nil, nil
		}
		return nil, fmt.Errorf("find password reset user: %w", err)
	}

	user := security.PltUserToModel(pltUser)

	token, err := generatePasswordResetToken()
	if err != nil {
		return nil, fmt.Errorf("generate reset token: %w", err)
	}
	expiresAt := time.Now().Add(1 * time.Hour)

	err = s.resetRepo.ReplacePasswordResetToken(ctx, user.ID, s.tokenHasher.Hash(token), expiresAt)
	if err != nil {
		return nil, fmt.Errorf("save reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/auth/reset/%s", s.appBaseURL, token)
	body := fmt.Sprintf("Use this link to reset your password:\n%s\n\nThis link expires in 1 hour.", resetURL)
	if err := s.emailService.Send(email, "Password Reset Request", body); err != nil {
		slog.Warn("Failed to send password reset email", "error", err)
	}

	actorID, actorName := userToAuditParams(user)
	s.auditor.Record(ctx, "PASSWORD_RESET_REQUESTED", actorID, actorName, nil, nil, "Password reset requested")

	return &token, nil
}

func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	normalized := strings.TrimSpace(newPassword)
	if err := validatePassword(normalized); err != nil {
		return err
	}

	hash, err := s.passwordEncoder.Encode(normalized)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	userID, err := s.resetRepo.ConsumePasswordResetToken(ctx, s.tokenHasher.Hash(token), hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.InvalidPasswordResetTokenError{}
		}
		return fmt.Errorf("consume password reset token: %w", err)
	}

	s.auditor.Record(ctx, "PASSWORD_RESET_COMPLETED", &userID, nil, nil, nil, "Password reset completed")

	return nil
}

func generatePasswordResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "prt_" + hex.EncodeToString(b), nil
}
