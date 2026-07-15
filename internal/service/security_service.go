package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
	"github.com/rygel/gouterstellar-platform/internal/security"
)

const (
	MinPasswordLength     = 8
	MaxUsernameLength     = 50
	SessionTokenHexLength = 48
	MaxPageLimit          = 1000
	MaxURLLength          = 2048
)

type SecurityService struct {
	userRepo              persistence.UserRepository
	passwordEncoder       security.PasswordEncoder
	sessionRepo           persistence.SessionRepository
	auditRepo             persistence.AuditRepository
	sessionTimeoutSeconds int64
}

func NewSecurityService(
	userRepo persistence.UserRepository,
	passwordEncoder security.PasswordEncoder,
	sessionRepo persistence.SessionRepository,
	auditRepo persistence.AuditRepository,
	sessionTimeoutSeconds int64,
) *SecurityService {
	return &SecurityService{
		userRepo:              userRepo,
		passwordEncoder:       passwordEncoder,
		sessionRepo:           sessionRepo,
		auditRepo:             auditRepo,
		sessionTimeoutSeconds: sessionTimeoutSeconds,
	}
}

func (s *SecurityService) Authenticate(ctx context.Context, username, password string) (*model.User, error) {
	pltUser, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	user := security.PltUserToModel(pltUser)

	if !user.Enabled {
		return nil, fmt.Errorf("account is disabled")
	}

	if !s.passwordEncoder.Matches(password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := s.userRepo.UpdateLastActivity(ctx, user.ID); err != nil {
		slog.Warn("Failed to update last activity", "userID", user.ID, "error", err)
	}

	return user, nil
}

func (s *SecurityService) Register(ctx context.Context, username, password string) (*model.User, error) {
	var validationErrors []string

	if strings.TrimSpace(username) == "" {
		validationErrors = append(validationErrors, "Username must not be blank")
	}
	if len(username) > MaxUsernameLength {
		validationErrors = append(validationErrors, fmt.Sprintf("Username must be at most %d characters", MaxUsernameLength))
	}
	if len(password) < MinPasswordLength {
		validationErrors = append(validationErrors, fmt.Sprintf("Password must be at least %d characters", MinPasswordLength))
	}
	if len(validationErrors) > 0 {
		return nil, &model.ValidationError{Errors: validationErrors}
	}

	_, err := s.userRepo.FindByUsername(ctx, username)
	if err == nil {
		return nil, &model.UsernameAlreadyExistsError{Username: username}
	}

	hash, err := s.passwordEncoder.Encode(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := uuid.New()
	pltUser, err := s.userRepo.CreateUser(ctx, userID, username, "", hash, string(model.RoleUser), true)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	user := security.PltUserToModel(pltUser)

	actorID, actorName := userToAuditParams(user)
	s.auditLog(ctx, actorID, actorName, nil, nil, "USER_REGISTER", "New user registered")

	return user, nil
}

func (s *SecurityService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	pltUser, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return &model.UserNotFoundError{UserID: userID.String()}
	}

	user := security.PltUserToModel(pltUser)

	if !s.passwordEncoder.Matches(currentPassword, user.PasswordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	if len(newPassword) < MinPasswordLength {
		return &model.WeakPasswordError{Message: fmt.Sprintf("Password must be at least %d characters", MinPasswordLength)}
	}

	hash, err := s.passwordEncoder.Encode(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = s.userRepo.CreateUser(ctx, user.ID, user.Username, user.Email, hash, string(user.Role), user.Enabled)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	actorID, actorName := userToAuditParams(user)
	s.auditLog(ctx, actorID, actorName, actorID, actorName, "PASSWORD_CHANGE", "Password changed")

	return nil
}

func (s *SecurityService) ListUsers(ctx context.Context) ([]model.UserSummary, error) {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	summaries := make([]model.UserSummary, len(users))
	for i, u := range users {
		summaries[i] = security.PltUserToModel(u).ToSummary()
	}
	return summaries, nil
}

func (s *SecurityService) ListUsersPaged(ctx context.Context, limit, offset int32) ([]model.UserSummary, error) {
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}

	users, err := s.userRepo.FindPage(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users paged: %w", err)
	}

	summaries := make([]model.UserSummary, len(users))
	for i, u := range users {
		summaries[i] = security.PltUserToModel(u).ToSummary()
	}
	return summaries, nil
}

func (s *SecurityService) CountUsers(ctx context.Context) (int64, error) {
	return s.userRepo.CountAll(ctx)
}

func (s *SecurityService) SetUserEnabled(ctx context.Context, adminID, targetID uuid.UUID, enabled bool) error {
	admin, err := s.userRepo.FindByID(ctx, adminID)
	if err != nil {
		return &model.UserNotFoundError{UserID: adminID.String()}
	}

	target, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return &model.UserNotFoundError{UserID: targetID.String()}
	}

	_, err = s.userRepo.UpdateEnabled(ctx, targetID, enabled)
	if err != nil {
		return fmt.Errorf("update enabled: %w", err)
	}

	adminModel := security.PltUserToModel(admin)
	targetModel := security.PltUserToModel(target)

	actorID, actorName := userToAuditParams(adminModel)
	targetIDPtr, targetName := userToAuditParams(targetModel)

	action := "USER_ENABLED"
	if !enabled {
		action = "USER_DISABLED"
	}

	s.auditLog(ctx, actorID, actorName, targetIDPtr, targetName, action, fmt.Sprintf("Set enabled=%v", enabled))

	return nil
}

func (s *SecurityService) SetUserRole(ctx context.Context, adminID, targetID uuid.UUID, role model.UserRole) error {
	admin, err := s.userRepo.FindByID(ctx, adminID)
	if err != nil {
		return &model.UserNotFoundError{UserID: adminID.String()}
	}

	target, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return &model.UserNotFoundError{UserID: targetID.String()}
	}

	_, err = s.userRepo.UpdateRole(ctx, targetID, string(role))
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}

	adminModel := security.PltUserToModel(admin)
	targetModel := security.PltUserToModel(target)

	actorID, actorName := userToAuditParams(adminModel)
	targetIDPtr, targetName := userToAuditParams(targetModel)
	s.auditLog(ctx, actorID, actorName, targetIDPtr, targetName, "ROLE_CHANGE", fmt.Sprintf("Role set to %s", role))

	_ = targetModel
	return nil
}

func (s *SecurityService) DevAdminID(ctx context.Context) uuid.UUID {
	users, err := s.ListUsersPaged(ctx, 1, 0)
	if err != nil {
		return uuid.Nil
	}
	for _, u := range users {
		if u.Role == string(model.RoleAdmin) {
			id, err := uuid.Parse(u.ID)
			if err != nil {
				return uuid.Nil
			}
			return id
		}
	}
	return uuid.Nil
}

func (s *SecurityService) CountAuditEntries(ctx context.Context) (int64, error) {
	return s.auditRepo.CountAll(ctx)
}

func (s *SecurityService) GetAuditLog(ctx context.Context, limit int32) ([]model.AuditEntry, error) {
	entries, err := s.auditRepo.FindRecent(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("get audit log: %w", err)
	}

	result := make([]model.AuditEntry, len(entries))
	for i, e := range entries {
		result[i] = pltAuditLogToModel(e)
	}
	return result, nil
}

func (s *SecurityService) GetAuditLogPaged(ctx context.Context, limit, offset int32) ([]model.AuditEntry, error) {
	entries, err := s.auditRepo.FindPage(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get audit log paged: %w", err)
	}

	result := make([]model.AuditEntry, len(entries))
	for i, e := range entries {
		result[i] = pltAuditLogToModel(e)
	}
	return result, nil
}

func (s *SecurityService) CreateSession(ctx context.Context, userID uuid.UUID) (string, error) {
	rawBytes := make([]byte, SessionTokenHexLength)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	rawToken := "oss_" + hex.EncodeToString(rawBytes)

	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	expiresAt := time.Now().Add(time.Duration(s.sessionTimeoutSeconds) * time.Second)

	_, err := s.sessionRepo.CreateSession(ctx, tokenHash, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return rawToken, nil
}

func (s *SecurityService) LookupSession(ctx context.Context, rawToken string) model.SessionLookup {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	session, err := s.sessionRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return model.SessionNotFound{}
	}

	sess := pltSessionToModel(session)

	if sess.ExpiresAt.Before(time.Now()) {
		return model.SessionExpired{}
	}

	newExpiresAt := time.Now().Add(time.Duration(s.sessionTimeoutSeconds) * time.Second)
	_, err = s.sessionRepo.UpdateExpiresAt(ctx, tokenHash, newExpiresAt)
	if err != nil {
		slog.Warn("Failed to extend session expiry", "error", err)
	}

	pltUser, err := s.userRepo.FindByID(ctx, sess.UserID)
	if err != nil {
		return model.SessionNotFound{}
	}

	user := security.PltUserToModel(pltUser)

	if !user.Enabled {
		return model.SessionNotFound{}
	}

	if err := s.userRepo.UpdateLastActivity(ctx, user.ID); err != nil {
		slog.Warn("Failed to update last activity", "userID", user.ID, "error", err)
	}

	return model.SessionActive{User: user}
}

func (s *SecurityService) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.sessionRepo.DeleteExpired(ctx)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

func (s *SecurityService) UpdateProfile(ctx context.Context, userID uuid.UUID, email string, username, avatarURL *string) error {
	pltUser, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return &model.UserNotFoundError{UserID: userID.String()}
	}

	user := security.PltUserToModel(pltUser)

	if strings.TrimSpace(email) == "" {
		return &model.ValidationError{Errors: []string{"Email must not be blank"}}
	}

	if username != nil && *username != user.Username {
		existing, err := s.userRepo.FindByUsername(ctx, *username)
		if err == nil && existing.ID != userID {
			return &model.UsernameAlreadyExistsError{Username: *username}
		}
	}

	if avatarURL != nil {
		if len(*avatarURL) > MaxURLLength {
			return &model.ValidationError{Errors: []string{fmt.Sprintf("Avatar URL must be at most %d characters", MaxURLLength)}}
		}
		if _, err := url.Parse(*avatarURL); err != nil {
			return &model.ValidationError{Errors: []string{"Invalid avatar URL"}}
		}
	}

	if username != nil {
		_, err = s.userRepo.UpdateUsername(ctx, userID, *username)
		if err != nil {
			return fmt.Errorf("update username: %w", err)
		}
	}

	if avatarURL != nil {
		_, err = s.userRepo.UpdateAvatarURL(ctx, userID, avatarURL)
		if err != nil {
			return fmt.Errorf("update avatar URL: %w", err)
		}
	}

	actorID, actorName := userToAuditParams(user)
	s.auditLog(ctx, actorID, actorName, nil, nil, "PROFILE_UPDATE", "Profile updated")

	return nil
}

func (s *SecurityService) DeleteAccount(ctx context.Context, userID uuid.UUID, currentPassword string) error {
	pltUser, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return &model.UserNotFoundError{UserID: userID.String()}
	}

	user := security.PltUserToModel(pltUser)

	if !s.passwordEncoder.Matches(currentPassword, user.PasswordHash) {
		return &model.WeakPasswordError{Message: "Current password is incorrect"}
	}

	if user.Role == model.RoleAdmin {
		adminCount, countErr := s.userRepo.CountByRole(ctx, string(model.RoleAdmin))
		if countErr != nil {
			return fmt.Errorf("count administrators: %w", countErr)
		}
		if adminCount <= 1 {
			return &model.InsufficientPermissionError{Message: "Cannot delete the only remaining admin account"}
		}
	}

	if err := s.userRepo.DeleteByID(ctx, userID); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	actorID, actorName := userToAuditParams(user)
	s.auditLog(ctx, actorID, actorName, actorID, actorName, "ACCOUNT_DELETED", "Account deleted")

	return nil
}

func (s *SecurityService) UpdateNotificationPreferences(ctx context.Context, userID uuid.UUID, emailEnabled, pushEnabled bool) error {
	_, err := s.userRepo.UpdateNotificationPreferences(ctx, userID, emailEnabled, pushEnabled)
	if err != nil {
		return fmt.Errorf("update notification preferences: %w", err)
	}
	return nil
}

func (s *SecurityService) UpdatePreferences(ctx context.Context, userID uuid.UUID, language, theme, layout *string) error {
	_, err := s.userRepo.UpdatePreferences(ctx, userID, language, theme, layout)
	if err != nil {
		return fmt.Errorf("update preferences: %w", err)
	}
	return nil
}

func (s *SecurityService) auditLog(ctx context.Context, actorID *uuid.UUID, actorUsername *string, targetID *uuid.UUID, targetUsername *string, action, detail string) {
	_, err := s.auditRepo.LogAudit(ctx, actorID, actorUsername, targetID, targetUsername, action, detail)
	if err != nil {
		slog.Error("Failed to log audit entry", "action", action, "error", err)
	}
}

func userToAuditParams(u *model.User) (*uuid.UUID, *string) {
	if u == nil {
		return nil, nil
	}
	id := u.ID
	name := u.Username
	return &id, &name
}

func pltSessionToModel(s db.PltSession) model.Session {
	return model.Session{
		ID:        s.ID,
		TokenHash: s.TokenHash,
		UserID:    s.UserID,
		CreatedAt: s.CreatedAt.Time,
		ExpiresAt: s.ExpiresAt.Time,
	}
}

func pltAuditLogToModel(e db.PltAuditLog) model.AuditEntry {
	var actorID *string
	if e.ActorID.Valid {
		s := uuid.UUID(e.ActorID.Bytes).String()
		actorID = &s
	}
	var targetID *string
	if e.TargetID.Valid {
		s := uuid.UUID(e.TargetID.Bytes).String()
		targetID = &s
	}
	return model.AuditEntry{
		ID:             e.ID,
		ActorID:        actorID,
		ActorUsername:  e.ActorUsername,
		TargetID:       targetID,
		TargetUsername: e.TargetUsername,
		Action:         e.Action,
		Detail:         e.Detail,
		CreatedAt:      e.CreatedAt.Time,
	}
}
