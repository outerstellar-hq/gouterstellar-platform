package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (db.PltUser, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (db.PltUser, error) {
	args := m.Called(ctx, username)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (db.PltUser, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) CreateUser(ctx context.Context, id uuid.UUID, username, email, passwordHash string, role string, enabled bool) (db.PltUser, error) {
	args := m.Called(ctx, id, username, email, passwordHash, role, enabled)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) FindAll(ctx context.Context) ([]db.PltUser, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.PltUser), args.Error(1)
}

func (m *mockUserRepo) FindPage(ctx context.Context, limit, offset int32) ([]db.PltUser, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]db.PltUser), args.Error(1)
}

func (m *mockUserRepo) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockUserRepo) CountByRole(ctx context.Context, role string) (int64, error) {
	args := m.Called(ctx, role)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockUserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role string) (db.PltUser, error) {
	args := m.Called(ctx, id, role)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) UpdateEnabled(ctx context.Context, id uuid.UUID, enabled bool) (db.PltUser, error) {
	args := m.Called(ctx, id, enabled)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) UpdateLastActivity(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepo) IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) (int32, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(int32), args.Error(1)
}

func (m *mockUserRepo) ResetLoginFailures(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepo) LockUserUntil(ctx context.Context, id uuid.UUID, until time.Time) error {
	args := m.Called(ctx, id, until)
	return args.Error(0)
}

func (m *mockUserRepo) DeleteByID(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepo) UpdateUsername(ctx context.Context, id uuid.UUID, username string) (db.PltUser, error) {
	args := m.Called(ctx, id, username)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) UpdateAvatarURL(ctx context.Context, id uuid.UUID, avatarURL *string) (db.PltUser, error) {
	args := m.Called(ctx, id, avatarURL)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) UpdateNotificationPreferences(ctx context.Context, id uuid.UUID, emailEnabled, pushEnabled bool) (db.PltUser, error) {
	args := m.Called(ctx, id, emailEnabled, pushEnabled)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) UpdatePreferences(ctx context.Context, id uuid.UUID, language, theme, layout *string) (db.PltUser, error) {
	args := m.Called(ctx, id, language, theme, layout)
	return args.Get(0).(db.PltUser), args.Error(1)
}

func (m *mockUserRepo) SeedAdminUser(ctx context.Context, id uuid.UUID, username, email, passwordHash string) (db.PltUser, error) {
	args := m.Called(ctx, id, username, email, passwordHash)
	return args.Get(0).(db.PltUser), args.Error(1)
}

type mockSessionRepo struct {
	mock.Mock
}

func (m *mockSessionRepo) CreateSession(ctx context.Context, tokenHash string, userID uuid.UUID, expiresAt time.Time) (db.PltSession, error) {
	args := m.Called(ctx, tokenHash, userID, expiresAt)
	return args.Get(0).(db.PltSession), args.Error(1)
}

func (m *mockSessionRepo) FindByTokenHash(ctx context.Context, tokenHash string) (db.PltSession, error) {
	args := m.Called(ctx, tokenHash)
	return args.Get(0).(db.PltSession), args.Error(1)
}

func (m *mockSessionRepo) FindByTokenHashIncludingExpired(ctx context.Context, tokenHash string) (db.PltSession, error) {
	args := m.Called(ctx, tokenHash)
	return args.Get(0).(db.PltSession), args.Error(1)
}

func (m *mockSessionRepo) UpdateExpiresAt(ctx context.Context, tokenHash string, expiresAt time.Time) (db.PltSession, error) {
	args := m.Called(ctx, tokenHash, expiresAt)
	return args.Get(0).(db.PltSession), args.Error(1)
}

func (m *mockSessionRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *mockSessionRepo) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockSessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

type mockPasswordEncoder struct {
	mock.Mock
}

func (m *mockPasswordEncoder) Encode(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *mockPasswordEncoder) Matches(password, hash string) bool {
	args := m.Called(password, hash)
	return args.Bool(0)
}

func makeTestUser(username, role string, enabled bool) db.PltUser {
	return db.PltUser{
		ID:           uuid.New(),
		Username:     username,
		Email:        username + "@test.com",
		PasswordHash: "hashedpassword",
		Role:         role,
		Enabled:      enabled,
		CreatedAt:    pgtype.Timestamptz{Valid: true},
	}
}

var testSecurityConfig = SecurityConfig{
	SessionTimeout: time.Hour, MaxFailedLoginAttempts: 10, LockoutDuration: 15 * time.Minute,
}

func TestAuthenticate_Success(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, testSecurityConfig)

	testUser := makeTestUser("alice", "USER", true)
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(testUser, nil)
	encoder.On("Matches", "password123", "hashedpassword").Return(true)
	userRepo.On("UpdateLastActivity", mock.Anything, testUser.ID).Return(nil)

	user, err := svc.Authenticate(context.Background(), "alice", "password123")

	assert.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.True(t, user.Enabled)
	userRepo.AssertExpectations(t)
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, testSecurityConfig)

	userRepo.On("FindByUsername", mock.Anything, "nonexistent").Return(db.PltUser{}, pgx.ErrNoRows)
	encoder.On("Encode", "timing-mitigation-dummy").Return("dummy-hash", nil)
	encoder.On("Matches", "password", "dummy-hash").Return(false)

	_, err := svc.Authenticate(context.Background(), "nonexistent", "password")

	assert.Error(t, err)
}

func TestAuthenticateFailsClosedWhenUserLookupFails(t *testing.T) {
	userRepo := new(mockUserRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), new(mockAuditRepo), testSecurityConfig)
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(db.PltUser{}, fmt.Errorf("database unavailable"))

	_, err := svc.Authenticate(context.Background(), "alice", "password")

	assert.Equal(t, errInvalidCredentials, err)
	encoder.AssertNotCalled(t, "Encode", mock.Anything)
}

func TestAuthenticate_AccountDisabled(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, testSecurityConfig)

	testUser := makeTestUser("bob", "USER", false)
	userRepo.On("FindByUsername", mock.Anything, "bob").Return(testUser, nil)
	encoder.On("Matches", "password", testUser.PasswordHash).Return(false)
	auditRepo.On("LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), "AUTHENTICATION_FAILED", "Invalid credentials").Return(db.PltAuditLog{}, nil)

	_, err := svc.Authenticate(context.Background(), "bob", "password")

	assert.Error(t, err)
	assert.Equal(t, errInvalidCredentials, err)
}

func TestAuthenticateLocksAccountAtConfiguredThreshold(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo, testSecurityConfig)
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }
	user := makeTestUser("alice", "USER", true)
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	userRepo.On("IncrementFailedLoginAttempts", mock.Anything, user.ID).Return(int32(10), nil)
	userRepo.On("LockUserUntil", mock.Anything, user.ID, fixedNow.Add(15*time.Minute)).Return(nil)
	encoder.On("Matches", "wrong-password", user.PasswordHash).Return(false)
	expectAuthenticationFailure(auditRepo)

	_, err := svc.Authenticate(context.Background(), "alice", "wrong-password")

	assert.Equal(t, errInvalidCredentials, err)
	userRepo.AssertExpectations(t)
}

func TestAuthenticateDoesNotLockAccountBelowConfiguredThreshold(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo, testSecurityConfig)
	user := makeTestUser("alice", "USER", true)
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	userRepo.On("IncrementFailedLoginAttempts", mock.Anything, user.ID).Return(int32(9), nil)
	encoder.On("Matches", "wrong-password", user.PasswordHash).Return(false)
	expectAuthenticationFailure(auditRepo)

	_, err := svc.Authenticate(context.Background(), "alice", "wrong-password")

	assert.Equal(t, errInvalidCredentials, err)
	userRepo.AssertNotCalled(t, "LockUserUntil", mock.Anything, mock.Anything, mock.Anything)
}

func TestAuthenticateRejectsLockedAccountWithoutCheckingPassword(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo, testSecurityConfig)
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }
	user := makeTestUser("alice", "USER", true)
	user.LockedUntil = pgtype.Timestamptz{Time: fixedNow.Add(time.Minute), Valid: true}
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	encoder.On("Matches", "correct-password", user.PasswordHash).Return(true)
	expectAuthenticationFailure(auditRepo)

	_, err := svc.Authenticate(context.Background(), "alice", "correct-password")

	assert.Equal(t, errInvalidCredentials, err)
	encoder.AssertNumberOfCalls(t, "Matches", 1)
}

func TestAuthenticateClearsExpiredLockAfterSuccessfulPassword(t *testing.T) {
	userRepo := new(mockUserRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), new(mockAuditRepo), testSecurityConfig)
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }
	user := makeTestUser("alice", "USER", true)
	user.FailedLoginAttempts = 10
	user.LockedUntil = pgtype.Timestamptz{Time: fixedNow.Add(-time.Minute), Valid: true}
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	userRepo.On("ResetLoginFailures", mock.Anything, user.ID).Return(nil)
	userRepo.On("UpdateLastActivity", mock.Anything, user.ID).Return(nil)
	encoder.On("Matches", "correct-password", user.PasswordHash).Return(true)

	authenticated, err := svc.Authenticate(context.Background(), "alice", "correct-password")

	assert.NoError(t, err)
	assert.Equal(t, user.ID, authenticated.ID)
	userRepo.AssertExpectations(t)
}

func expectAuthenticationFailure(auditRepo *mockAuditRepo) {
	auditRepo.On(
		"LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"),
		mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), "AUTHENTICATION_FAILED", "Invalid credentials",
	).Return(db.PltAuditLog{}, nil)
}

func TestUnlockAccountResetsFailuresAndAudits(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	svc := NewSecurityService(userRepo, new(mockPasswordEncoder), new(mockSessionRepo), auditRepo, testSecurityConfig)
	admin := makeTestUser("admin", "ADMIN", true)
	target := makeTestUser("alice", "USER", true)
	userRepo.On("FindByID", mock.Anything, admin.ID).Return(admin, nil)
	userRepo.On("FindByID", mock.Anything, target.ID).Return(target, nil)
	userRepo.On("ResetLoginFailures", mock.Anything, target.ID).Return(nil)
	auditRepo.On(
		"LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"),
		mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), "USER_UNLOCKED", "Account unlocked",
	).Return(db.PltAuditLog{}, nil)

	err := svc.UnlockAccount(context.Background(), admin.ID, target.ID)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUnlockAccountRejectsNonAdministrator(t *testing.T) {
	userRepo := new(mockUserRepo)
	svc := NewSecurityService(userRepo, new(mockPasswordEncoder), new(mockSessionRepo), new(mockAuditRepo), testSecurityConfig)
	user := makeTestUser("alice", "USER", true)
	userRepo.On("FindByID", mock.Anything, user.ID).Return(user, nil)

	err := svc.UnlockAccount(context.Background(), user.ID, uuid.New())

	assert.IsType(t, &model.InsufficientPermissionError{}, err)
}

func TestRegister_Success(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, testSecurityConfig)

	userRepo.On("FindByUsername", mock.Anything, "newuser").Return(db.PltUser{}, fmt.Errorf("not found"))
	encoder.On("Encode", "password123").Return("hashed", nil)
	userRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("uuid.UUID"), "newuser", "", "hashed", "USER", true).Return(makeTestUser("newuser", "USER", true), nil)
	auditRepo.On("LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_REGISTER", "New user registered").Return(db.PltAuditLog{}, nil)

	user, err := svc.Register(context.Background(), "newuser", "password123")

	assert.NoError(t, err)
	assert.Equal(t, "newuser", user.Username)
	userRepo.AssertExpectations(t)
}

func TestRegister_ShortPassword(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, testSecurityConfig)

	_, err := svc.Register(context.Background(), "newuser", "short")

	assert.Error(t, err)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, testSecurityConfig)

	userRepo.On("FindByUsername", mock.Anything, "existing").Return(makeTestUser("existing", "USER", true), nil)

	_, err := svc.Register(context.Background(), "existing", "password123")

	assert.Error(t, err)
}

func TestDeleteAccountRejectsIncorrectPassword(t *testing.T) {
	userRepo := new(mockUserRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), new(mockAuditRepo), testSecurityConfig)
	user := makeTestUser("alice", "USER", true)
	userRepo.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	encoder.On("Matches", "wrong-password", user.PasswordHash).Return(false)

	err := svc.DeleteAccount(context.Background(), user.ID, "wrong-password")

	assert.IsType(t, &model.WeakPasswordError{}, err)
	userRepo.AssertNotCalled(t, "DeleteByID", mock.Anything, mock.Anything)
}

func TestDeleteAccountProtectsOnlyAdministrator(t *testing.T) {
	userRepo := new(mockUserRepo)
	encoder := new(mockPasswordEncoder)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), new(mockAuditRepo), testSecurityConfig)
	user := makeTestUser("admin", "ADMIN", true)
	userRepo.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	userRepo.On("CountByRole", mock.Anything, "ADMIN").Return(int64(1), nil)
	encoder.On("Matches", "correct-password", user.PasswordHash).Return(true)

	err := svc.DeleteAccount(context.Background(), user.ID, "correct-password")

	assert.IsType(t, &model.InsufficientPermissionError{}, err)
	userRepo.AssertNotCalled(t, "DeleteByID", mock.Anything, mock.Anything)
}

func TestDeleteAccountRemovesVerifiedUser(t *testing.T) {
	userRepo := new(mockUserRepo)
	encoder := new(mockPasswordEncoder)
	auditRepo := new(mockAuditRepo)
	svc := NewSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo, testSecurityConfig)
	user := makeTestUser("alice", "USER", true)
	userRepo.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	userRepo.On("DeleteByID", mock.Anything, user.ID).Return(nil)
	encoder.On("Matches", "correct-password", user.PasswordHash).Return(true)
	auditRepo.On(
		"LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"),
		mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), "ACCOUNT_DELETED", "Account deleted",
	).Return(db.PltAuditLog{}, nil)

	err := svc.DeleteAccount(context.Background(), user.ID, "correct-password")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}
