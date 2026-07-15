package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
	"github.com/rygel/gouterstellar-platform/internal/security"
)

var errInvalidCredentials = errors.New("invalid credentials")

type SecurityConfig struct {
	SessionTimeout         time.Duration
	MaxFailedLoginAttempts int32
	LockoutDuration        time.Duration
}

type SecurityDependencies struct {
	UserRepository      persistence.UserRepository
	PasswordEncoder     security.PasswordEncoder
	SessionRepository   persistence.SessionRepository
	AuditRepository     persistence.AuditRepository
	NotificationService *NotificationService
	EmailService        EmailService
	TOTPService         *TOTPService
}

const (
	MinPasswordLength     = 8
	MaxUsernameLength     = 50
	SessionTokenHexLength = 48
	MaxPageLimit          = 1000
	MaxURLLength          = 2048
)

type SecurityService struct {
	userRepo            persistence.UserRepository
	passwordEncoder     security.PasswordEncoder
	sessionRepo         persistence.SessionRepository
	auditRepo           persistence.AuditRepository
	auditor             Auditor
	notificationService *NotificationService
	emailService        EmailService
	totpService         *TOTPService
	config              SecurityConfig
	now                 func() time.Time
	dummyHashOnce       sync.Once
	dummyPasswordHash   string
}

func NewSecurityService(
	dependencies SecurityDependencies,
	config SecurityConfig,
) *SecurityService {
	return &SecurityService{
		userRepo:        dependencies.UserRepository,
		passwordEncoder: dependencies.PasswordEncoder,
		sessionRepo:     dependencies.SessionRepository,
		auditRepo:       dependencies.AuditRepository,
		// The auditor wraps the same repo so the write path is uniform with the
		// other services. auditRepo is retained for the audit-read methods
		// (GetAuditLog/GetAuditLogPaged/CountAuditEntries).
		auditor:             NewAuditService(dependencies.AuditRepository),
		notificationService: dependencies.NotificationService,
		emailService:        dependencies.EmailService,
		totpService:         dependencies.TOTPService,
		config:              config,
		now:                 time.Now,
	}
}

func (s *SecurityService) Authenticate(ctx context.Context, username, password string) (model.AuthenticationResult, error) {
	pltUser, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("Failed to look up user during authentication", "error", err)
			return nil, errInvalidCredentials
		}
		s.dummyHashOnce.Do(func() {
			hash, hashErr := s.passwordEncoder.Encode("outerstellar-dummy-password")
			if hashErr == nil {
				s.dummyPasswordHash = hash
			}
		})
		if s.dummyPasswordHash != "" {
			s.passwordEncoder.Matches(password, s.dummyPasswordHash)
		}
		attempted := username
		s.AuditLoginFailed(ctx, &attempted)
		return nil, errInvalidCredentials
	}

	user := security.PltUserToModel(pltUser)
	passwordMatches := s.passwordEncoder.Matches(password, user.PasswordHash)
	locked := user.LockedUntil != nil && user.LockedUntil.After(s.now())

	if !user.Enabled || locked {
		attempted := username
		s.AuditLoginFailed(ctx, &attempted)
		return nil, errInvalidCredentials
	}

	if !passwordMatches {
		attempts, updateErr := s.userRepo.IncrementFailedLoginAttempts(ctx, user.ID)
		if updateErr != nil {
			slog.Error("Failed to record login failure", "userID", user.ID, "error", updateErr)
			return nil, errInvalidCredentials
		}
		if attempts >= s.config.MaxFailedLoginAttempts {
			until := s.now().Add(s.config.LockoutDuration)
			if lockErr := s.userRepo.LockUserUntil(ctx, user.ID, until); lockErr != nil {
				slog.Error("Failed to lock account", "userID", user.ID, "error", lockErr)
				return nil, errInvalidCredentials
			}
		}
		attempted := username
		s.AuditLoginFailed(ctx, &attempted)
		return nil, errInvalidCredentials
	}

	if user.FailedLoginAttempts > 0 || user.LockedUntil != nil {
		if err := s.userRepo.ResetLoginFailures(ctx, user.ID); err != nil {
			slog.Error("Failed to clear login failures", "userID", user.ID, "error", err)
			return nil, errInvalidCredentials
		}
	}

	if user.TOTPEnabled {
		if s.totpService == nil {
			slog.Error("TOTP is enabled for user but the service is unavailable", "userID", user.ID)
			return nil, errInvalidCredentials
		}
		partialToken, err := s.totpService.CreateChallenge(ctx, user.ID)
		if err != nil {
			return nil, errInvalidCredentials
		}
		return model.TOTPRequired{PartialToken: partialToken}, nil
	}

	if err := s.userRepo.UpdateLastActivity(ctx, user.ID); err != nil {
		slog.Warn("Failed to update last activity", "userID", user.ID, "error", err)
	}
	s.AuditLogin(ctx, &user.ID, &user.Username)

	return model.Authenticated{User: user}, nil
}

func (s *SecurityService) UnlockAccount(ctx context.Context, adminID, targetID uuid.UUID) error {
	admin, err := s.userRepo.FindByID(ctx, adminID)
	if err != nil {
		return &model.UserNotFoundError{UserID: adminID.String()}
	}
	adminModel := security.PltUserToModel(admin)
	if adminModel.Role != model.RoleAdmin {
		return fmt.Errorf("admin access required")
	}

	target, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return &model.UserNotFoundError{UserID: targetID.String()}
	}
	if err := s.userRepo.ResetLoginFailures(ctx, targetID); err != nil {
		return fmt.Errorf("unlock account: %w", err)
	}
	if s.totpService != nil {
		if err := s.totpService.ResetFailures(ctx, targetID); err != nil {
			return fmt.Errorf("unlock account: %w", err)
		}
	}

	targetModel := security.PltUserToModel(target)
	actorID, actorName := userToAuditParams(adminModel)
	targetIDPtr, targetName := userToAuditParams(targetModel)
	s.auditLog(ctx, actorID, actorName, targetIDPtr, targetName, "USER_UNLOCKED", "Account unlocked")
	return nil
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

	// Send a welcome email regardless of notification preference — the user just
	// signed up, so this transactional message is expected. Failures are logged
	// but never block registration.
	if s.emailService != nil && user.Email != "" {
		if err := s.emailService.Send(user.Email, "Welcome to Outerstellar", "Your account has been created."); err != nil {
			slog.Error("Failed to send welcome email", "userID", user.ID, "error", err)
		}
	}

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

	err = s.userRepo.UpdatePasswordHash(ctx, userID, hash)
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

	if !enabled {
		if s.notificationService != nil {
			if err := s.notificationService.Create(ctx, targetID, "Account Disabled", "Your account has been disabled by an administrator.", "account"); err != nil {
				slog.Warn("Failed to create account-disabled notification", "userID", targetID, "error", err)
			}
		}
		// Notify the affected user by email when they have email notifications
		// enabled. Synchronous send is acceptable for the PoC; failures are
		// logged but never block the disable action.
		if s.emailService != nil && targetModel.EmailNotificationsEnabled && targetModel.Email != "" {
			if err := s.emailService.Send(targetModel.Email, "Account Disabled", "Your account has been disabled by an administrator."); err != nil {
				slog.Error("Failed to send account-disabled email", "userID", targetID, "error", err)
			}
		}
	}

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

	if s.notificationService != nil {
		body := fmt.Sprintf("Your role has been changed to %s.", role)
		if err := s.notificationService.Create(ctx, targetID, "Role Updated", body, "account"); err != nil {
			slog.Warn("Failed to create role-change notification", "userID", targetID, "error", err)
		}
	}

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

	tokenHash := hashToken(rawToken)

	expiresAt := s.now().Add(s.config.SessionTimeout)

	_, err := s.sessionRepo.CreateSession(ctx, tokenHash, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return rawToken, nil
}

func (s *SecurityService) LookupSession(ctx context.Context, rawToken string) model.SessionLookup {
	tokenHash := hashToken(rawToken)

	session, err := s.sessionRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return model.SessionNotFound{}
	}

	sess := pltSessionToModel(session)

	if sess.ExpiresAt.Before(s.now()) {
		return model.SessionExpired{}
	}

	newExpiresAt := s.now().Add(s.config.SessionTimeout)
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

// Logout invalidates a session by its raw token and records an audit event when
// an actor is supplied. It is a no-op for an empty token so it is safe to call
// unconditionally from logout handlers. The actor parameters (userID/username)
// are optional: pass nil to skip the audit entry, matching the prior handler
// behavior where a logout without a known user was logged without an actor.
func (s *SecurityService) Logout(ctx context.Context, rawToken string, actorID *uuid.UUID, actorName *string) error {
	if rawToken == "" {
		return nil
	}
	tokenHash := hashToken(rawToken)
	if err := s.sessionRepo.DeleteByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	s.AuditLogout(ctx, actorID, actorName)
	return nil
}

func (s *SecurityService) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.sessionRepo.DeleteExpired(ctx)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	if s.totpService != nil {
		if err := s.totpService.DeleteExpiredChallenges(ctx); err != nil {
			return err
		}
	}
	return nil
}

// SessionInfo is a UI-facing summary of an active session. The token hash is
// masked so only a short prefix is exposed; the full hash never leaves the
// store but is carried on the struct so the UI can address a session for
// revocation.
type SessionInfo struct {
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// MaskedTokenHash returns the first 8 characters of the session token hash,
// enough to distinguish sessions without leaking the full hash.
func (s SessionInfo) MaskedTokenHash() string {
	if len(s.TokenHash) <= 8 {
		return s.TokenHash
	}
	return s.TokenHash[:8]
}

// ListUserSessions returns the active (non-expired) sessions for the given
// user, most recently created first.
func (s *SecurityService) ListUserSessions(ctx context.Context, userID uuid.UUID) ([]SessionInfo, error) {
	rows, err := s.sessionRepo.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user sessions: %w", err)
	}

	result := make([]SessionInfo, len(rows))
	for i, row := range rows {
		result[i] = SessionInfo{
			TokenHash: row.TokenHash,
			CreatedAt: row.CreatedAt.Time,
			ExpiresAt: row.ExpiresAt.Time,
		}
	}
	return result, nil
}

// RevokeSession deletes a single session by its token hash, invalidating the
// associated cookie on its next lookup.
func (s *SecurityService) RevokeSession(ctx context.Context, tokenHash string) error {
	if err := s.sessionRepo.DeleteByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
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

func (s *SecurityService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	pltUser, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return &model.UserNotFoundError{UserID: userID.String()}
	}

	user := security.PltUserToModel(pltUser)

	if err := s.sessionRepo.DeleteByUserID(ctx, userID); err != nil {
		slog.Warn("Failed to delete user sessions", "userID", userID, "error", err)
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
	if s.auditor != nil {
		s.auditor.Record(ctx, action, actorID, actorUsername, targetID, targetUsername, detail)
	}
}

// AuditLogin records a successful login audit event.
func (s *SecurityService) AuditLogin(ctx context.Context, userID *uuid.UUID, username *string) {
	s.auditLog(ctx, userID, username, nil, nil, "USER_LOGIN", "Login successful")
}

// AuditLoginFailed records a failed login attempt for the given username.
func (s *SecurityService) AuditLoginFailed(ctx context.Context, username *string) {
	s.auditLog(ctx, nil, username, nil, nil, "USER_LOGIN_FAILED", "Login failed")
}

// AuditLogout records a logout audit event.
func (s *SecurityService) AuditLogout(ctx context.Context, userID *uuid.UUID, username *string) {
	s.auditLog(ctx, userID, username, nil, nil, "USER_LOGOUT", "Logout")
}

// hashToken returns the SHA-256 hash of a raw session token, hex-encoded.
// This is the stored form of session tokens; the raw token is never persisted.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
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
