package service

import (
	"context"
	"errors"
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

func (m *mockUserRepo) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
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

func (m *mockSessionRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]db.ListSessionsForUserRow, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]db.ListSessionsForUserRow), args.Error(1)
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

func testSecurityConfig() SecurityConfig {
	return SecurityConfig{
		SessionTimeout:         time.Hour,
		MaxFailedLoginAttempts: 10,
		LockoutDuration:        15 * time.Minute,
	}
}

func newTestSecurityService(userRepo *mockUserRepo, encoder *mockPasswordEncoder, sessionRepo *mockSessionRepo, auditRepo *mockAuditRepo) *SecurityService {
	return NewSecurityService(SecurityDependencies{
		UserRepository:    userRepo,
		PasswordEncoder:   encoder,
		SessionRepository: sessionRepo,
		AuditRepository:   auditRepo,
	}, testSecurityConfig())
}

func TestAuthenticate_Success(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := newTestSecurityService(userRepo, encoder, sessionRepo, auditRepo)

	testUser := makeTestUser("alice", "USER", true)
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(testUser, nil)
	encoder.On("Matches", "password123", "hashedpassword").Return(true)
	userRepo.On("UpdateLastActivity", mock.Anything, testUser.ID).Return(nil)
	auditRepo.On("LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_LOGIN", "Login successful").Return(db.PltAuditLog{}, nil)

	result, err := svc.Authenticate(context.Background(), "alice", "password123")

	assert.NoError(t, err)
	user := result.(model.Authenticated).User
	assert.Equal(t, "alice", user.Username)
	assert.True(t, user.Enabled)
	userRepo.AssertExpectations(t)
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := newTestSecurityService(userRepo, encoder, sessionRepo, auditRepo)

	userRepo.On("FindByUsername", mock.Anything, "nonexistent").Return(db.PltUser{}, pgx.ErrNoRows)
	encoder.On("Encode", "outerstellar-dummy-password").Return("dummy-hash", nil)
	encoder.On("Matches", "password", "dummy-hash").Return(false)
	auditRepo.On("LogAudit", mock.Anything, (*uuid.UUID)(nil), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_LOGIN_FAILED", "Login failed").Return(db.PltAuditLog{}, nil)

	_, err := svc.Authenticate(context.Background(), "nonexistent", "password")

	assert.Error(t, err)
}

func TestAuthenticate_AccountDisabled(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := newTestSecurityService(userRepo, encoder, sessionRepo, auditRepo)

	testUser := makeTestUser("bob", "USER", false)
	userRepo.On("FindByUsername", mock.Anything, "bob").Return(testUser, nil)
	encoder.On("Matches", "password", "hashedpassword").Return(false)
	auditRepo.On("LogAudit", mock.Anything, (*uuid.UUID)(nil), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_LOGIN_FAILED", "Login failed").Return(db.PltAuditLog{}, nil)

	_, err := svc.Authenticate(context.Background(), "bob", "password")

	assert.Error(t, err)
	assert.Equal(t, errInvalidCredentials, err)
}

func TestAuthenticate_LocksAtFailureThreshold(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	svc := newTestSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	user := makeTestUser("alice", "USER", true)
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	encoder.On("Matches", "wrong", user.PasswordHash).Return(false)
	userRepo.On("IncrementFailedLoginAttempts", mock.Anything, user.ID).Return(int32(10), nil)
	userRepo.On("LockUserUntil", mock.Anything, user.ID, now.Add(15*time.Minute)).Return(nil)
	auditRepo.On("LogAudit", mock.Anything, (*uuid.UUID)(nil), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_LOGIN_FAILED", "Login failed").Return(db.PltAuditLog{}, nil)

	_, err := svc.Authenticate(context.Background(), "alice", "wrong")

	assert.ErrorIs(t, err, errInvalidCredentials)
	userRepo.AssertExpectations(t)
}

func TestAuthenticate_RejectsLockedAccount(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	svc := newTestSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	user := makeTestUser("alice", "USER", true)
	user.LockedUntil = pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true}
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	encoder.On("Matches", "password123", user.PasswordHash).Return(true)
	auditRepo.On("LogAudit", mock.Anything, (*uuid.UUID)(nil), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_LOGIN_FAILED", "Login failed").Return(db.PltAuditLog{}, nil)

	_, err := svc.Authenticate(context.Background(), "alice", "password123")

	assert.ErrorIs(t, err, errInvalidCredentials)
	userRepo.AssertNotCalled(t, "IncrementFailedLoginAttempts", mock.Anything, mock.Anything)
}

func TestAuthenticate_ClearsExpiredLockAfterSuccess(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	svc := newTestSecurityService(userRepo, encoder, new(mockSessionRepo), auditRepo)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	user := makeTestUser("alice", "USER", true)
	user.FailedLoginAttempts = 10
	user.LockedUntil = pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	encoder.On("Matches", "password123", user.PasswordHash).Return(true)
	userRepo.On("ResetLoginFailures", mock.Anything, user.ID).Return(nil)
	userRepo.On("UpdateLastActivity", mock.Anything, user.ID).Return(nil)
	auditRepo.On("LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), (*uuid.UUID)(nil), (*string)(nil), "USER_LOGIN", "Login successful").Return(db.PltAuditLog{}, nil)

	result, err := svc.Authenticate(context.Background(), "alice", "password123")

	assert.NoError(t, err)
	assert.Equal(t, user.ID, result.(model.Authenticated).User.ID)
	userRepo.AssertExpectations(t)
}

func TestAuthenticate_DatabaseFailureFailsClosed(t *testing.T) {
	userRepo := new(mockUserRepo)
	svc := newTestSecurityService(userRepo, new(mockPasswordEncoder), new(mockSessionRepo), new(mockAuditRepo))
	userRepo.On("FindByUsername", mock.Anything, "alice").Return(db.PltUser{}, errors.New("database unavailable"))

	_, err := svc.Authenticate(context.Background(), "alice", "password123")

	assert.ErrorIs(t, err, errInvalidCredentials)
}

func TestUnlockAccount_ResetsFailuresAndAudits(t *testing.T) {
	userRepo := new(mockUserRepo)
	auditRepo := new(mockAuditRepo)
	svc := newTestSecurityService(userRepo, new(mockPasswordEncoder), new(mockSessionRepo), auditRepo)
	totpRepo := new(fakeTOTPRepo)
	svc.totpService = newTOTPTestService(totpRepo, userRepo)
	admin := makeTestUser("admin", "ADMIN", true)
	target := makeTestUser("alice", "USER", true)
	userRepo.On("FindByID", mock.Anything, admin.ID).Return(admin, nil)
	userRepo.On("FindByID", mock.Anything, target.ID).Return(target, nil)
	userRepo.On("ResetLoginFailures", mock.Anything, target.ID).Return(nil)
	auditRepo.On("LogAudit", mock.Anything, mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), mock.AnythingOfType("*uuid.UUID"), mock.AnythingOfType("*string"), "USER_UNLOCKED", "Account unlocked").Return(db.PltAuditLog{}, nil)

	err := svc.UnlockAccount(context.Background(), admin.ID, target.ID)

	assert.NoError(t, err)
	assert.True(t, totpRepo.reset)
	userRepo.AssertExpectations(t)
}

func TestRegister_Success(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := newTestSecurityService(userRepo, encoder, sessionRepo, auditRepo)

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

	svc := newTestSecurityService(userRepo, encoder, sessionRepo, auditRepo)

	_, err := svc.Register(context.Background(), "newuser", "short")

	assert.Error(t, err)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := newTestSecurityService(userRepo, encoder, sessionRepo, auditRepo)

	userRepo.On("FindByUsername", mock.Anything, "existing").Return(makeTestUser("existing", "USER", true), nil)

	_, err := svc.Register(context.Background(), "existing", "password123")

	assert.Error(t, err)
}
