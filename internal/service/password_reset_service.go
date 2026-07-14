package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/security"
)

type PasswordResetService struct {
	userRepo        persistence.UserRepository
	passwordEncoder security.PasswordEncoder
	resetRepo       persistence.PasswordResetRepository
	emailService    EmailService
	auditRepo       persistence.AuditRepository
	appBaseURL      string
}

func NewPasswordResetService(
	userRepo persistence.UserRepository,
	passwordEncoder security.PasswordEncoder,
	resetRepo persistence.PasswordResetRepository,
	emailService EmailService,
	auditRepo persistence.AuditRepository,
	appBaseURL string,
) *PasswordResetService {
	return &PasswordResetService{
		userRepo:        userRepo,
		passwordEncoder: passwordEncoder,
		resetRepo:       resetRepo,
		emailService:    emailService,
		auditRepo:       auditRepo,
		appBaseURL:      appBaseURL,
	}
}

func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email string) (*string, error) {
	pltUser, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		slog.Info("Password reset requested for unknown email", "email", email)
		return nil, nil
	}

	user := security.PltUserToModel(pltUser)

	token := uuid.New().String()
	expiresAt := time.Now().Add(1 * time.Hour)

	_, err = s.resetRepo.SavePasswordResetToken(ctx, user.ID, token, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("save reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/auth/reset/confirm?token=%s", s.appBaseURL, token)
	body := fmt.Sprintf("Click the following link to reset your password: %s", resetURL)
	s.emailService.Send(email, "Password Reset", body)

	actorID, actorName := userToAuditParams(user)
	s.auditLog(ctx, actorID, actorName, nil, nil, "PASSWORD_RESET_REQUESTED", "Password reset requested")

	return &token, nil
}

func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	resetToken, err := s.resetRepo.FindByToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	if resetToken.Used {
		return fmt.Errorf("reset token has already been used")
	}

	if resetToken.ExpiresAt.Time.Before(time.Now()) {
		return fmt.Errorf("reset token has expired")
	}

	if len(newPassword) < MinPasswordLength {
		return &model.WeakPasswordError{Message: fmt.Sprintf("Password must be at least %d characters", MinPasswordLength)}
	}

	pltUser, err := s.userRepo.FindByID(ctx, resetToken.UserID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	user := security.PltUserToModel(pltUser)

	hash, err := s.passwordEncoder.Encode(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.userRepo.UpdatePasswordHash(ctx, user.ID, hash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_, err = s.resetRepo.MarkUsed(ctx, token)
	if err != nil {
		slog.Error("Failed to mark reset token as used", "error", err)
	}

	actorID, actorName := userToAuditParams(user)
	s.auditLog(ctx, actorID, actorName, nil, nil, "PASSWORD_RESET_COMPLETED", "Password reset completed")

	return nil
}

func (s *PasswordResetService) auditLog(ctx context.Context, actorID *uuid.UUID, actorUsername *string, targetID *uuid.UUID, targetUsername *string, action, detail string) {
	_, err := s.auditRepo.LogAudit(ctx, actorID, actorUsername, targetID, targetUsername, action, detail)
	if err != nil {
		slog.Error("Failed to log audit entry", "action", action, "error", err)
	}
}
