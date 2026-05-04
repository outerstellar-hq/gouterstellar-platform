package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestAuthenticate_Success(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, 3600)

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

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, 3600)

	userRepo.On("FindByUsername", mock.Anything, "nonexistent").Return(db.PltUser{}, fmt.Errorf("not found"))

	_, err := svc.Authenticate(context.Background(), "nonexistent", "password")

	assert.Error(t, err)
}

func TestAuthenticate_AccountDisabled(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, 3600)

	testUser := makeTestUser("bob", "USER", false)
	userRepo.On("FindByUsername", mock.Anything, "bob").Return(testUser, nil)

	_, err := svc.Authenticate(context.Background(), "bob", "password")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestRegister_Success(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, 3600)

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

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, 3600)

	_, err := svc.Register(context.Background(), "newuser", "short")

	assert.Error(t, err)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	userRepo := new(mockUserRepo)
	sessionRepo := new(mockSessionRepo)
	auditRepo := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)

	svc := NewSecurityService(userRepo, encoder, sessionRepo, auditRepo, 3600)

	userRepo.On("FindByUsername", mock.Anything, "existing").Return(makeTestUser("existing", "USER", true), nil)

	_, err := svc.Register(context.Background(), "existing", "password123")

	assert.Error(t, err)
}
