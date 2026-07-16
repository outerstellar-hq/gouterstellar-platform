package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pquerna/otp"
	otplib "github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/security"
)

type fakeTOTPRepo struct {
	challenge          db.PltTotpChallenge
	challengeHash      string
	challengeExpires   time.Time
	deleteResult       bool
	deleted            bool
	enabledSecret      string
	enabledBackupCodes string
	disabled           bool
	failedAttempts     int32
	reset              bool
	replaceResult      bool
	replacement        *string
}

func (f *fakeTOTPRepo) CreateChallenge(_ context.Context, tokenHash string, userID uuid.UUID, expiresAt time.Time) error {
	f.challengeHash = tokenHash
	f.challengeExpires = expiresAt
	f.challenge = db.PltTotpChallenge{TokenHash: tokenHash, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (f *fakeTOTPRepo) TakeChallengeAttempt(_ context.Context, tokenHash string, _ int32) (db.PltTotpChallenge, error) {
	if f.challenge.TokenHash == "" || tokenHash != f.challenge.TokenHash {
		return db.PltTotpChallenge{}, pgx.ErrNoRows
	}
	return f.challenge, nil
}

func (f *fakeTOTPRepo) DeleteChallenge(_ context.Context, tokenHash string) (bool, error) {
	f.deleted = true
	if f.deleteResult && tokenHash == f.challenge.TokenHash {
		f.challenge = db.PltTotpChallenge{}
		return true, nil
	}
	return false, nil
}

func (f *fakeTOTPRepo) DeleteExpiredChallenges(context.Context) (int64, error) { return 0, nil }

func (f *fakeTOTPRepo) Enable(_ context.Context, _ uuid.UUID, secret, backupCodes string) error {
	f.enabledSecret = secret
	f.enabledBackupCodes = backupCodes
	return nil
}

func (f *fakeTOTPRepo) Disable(context.Context, uuid.UUID) error {
	f.disabled = true
	return nil
}

func (f *fakeTOTPRepo) IncrementFailedAttempts(context.Context, uuid.UUID) (int32, error) {
	return f.failedAttempts, nil
}

func (f *fakeTOTPRepo) ResetFailedAttempts(context.Context, uuid.UUID) error {
	f.reset = true
	return nil
}

func (f *fakeTOTPRepo) ReplaceBackupCodes(_ context.Context, _ uuid.UUID, _ string, replacement *string) (bool, error) {
	f.replacement = replacement
	return f.replaceResult, nil
}

func newTOTPTestService(repo *fakeTOTPRepo, users *mockUserRepo) *TOTPService {
	return NewTOTPService(repo, users, security.NewBCryptPasswordEncoder(4), nil, testSecurityConfig())
}

func generateTOTPCode(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	code, err := otplib.GenerateCodeCustom(secret, now, otplib.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	require.NoError(t, err)
	return code
}

func invalidTOTPCode(t *testing.T, secret string, now time.Time) string {
	valid := generateTOTPCode(t, secret, now)
	replacement := byte('0')
	if valid[len(valid)-1] == replacement {
		replacement = '1'
	}
	return valid[:len(valid)-1] + string(replacement)
}

func TestTOTPSetupAndEnrollment(t *testing.T) {
	repo := new(fakeTOTPRepo)
	svc := newTOTPTestService(repo, new(mockUserRepo))
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	setup, err := svc.GenerateSetup("alice@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, setup.Secret)
	assert.True(t, strings.HasPrefix(setup.QRDataURI, "data:image/png;base64,"))

	rawCodes, err := svc.ConfirmEnrollment(context.Background(), uuid.New(), setup.Secret, generateTOTPCode(t, setup.Secret, now))
	require.NoError(t, err)
	require.Len(t, rawCodes, totpBackupCodeCount)
	assert.Equal(t, setup.Secret, repo.enabledSecret)
	for _, code := range rawCodes {
		assert.NotContains(t, repo.enabledBackupCodes, code)
	}
	var hashes []string
	require.NoError(t, json.Unmarshal([]byte(repo.enabledBackupCodes), &hashes))
	assert.Len(t, hashes, totpBackupCodeCount)
}

func TestTOTPBackupCodeIsSingleUse(t *testing.T) {
	svc := newTOTPTestService(new(fakeTOTPRepo), new(mockUserRepo))
	rawCodes, encoded, err := svc.generateBackupCodes()
	require.NoError(t, err)

	matched, replacement, err := svc.consumeBackupCode(rawCodes[0], encoded)
	require.NoError(t, err)
	require.True(t, matched)
	require.NotNil(t, replacement)

	matchedAgain, _, err := svc.consumeBackupCode(rawCodes[0], *replacement)
	require.NoError(t, err)
	assert.False(t, matchedAgain)
}

func TestVerifyTOTPChallengeConsumesToken(t *testing.T) {
	repo := &fakeTOTPRepo{deleteResult: true}
	users := new(mockUserRepo)
	svc := newTOTPTestService(repo, users)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	setup, err := svc.GenerateSetup("alice@example.com")
	require.NoError(t, err)
	user := makeTestUser("alice", "USER", true)
	user.TotpEnabled = true
	user.TotpSecret = &setup.Secret
	repo.challenge = db.PltTotpChallenge{TokenHash: hashToken("pt_valid"), UserID: user.ID, AttemptCount: 1}
	users.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	users.On("UpdateLastActivity", mock.Anything, user.ID).Return(nil)

	result, err := svc.VerifyChallenge(context.Background(), "pt_valid", generateTOTPCode(t, setup.Secret, now))

	require.NoError(t, err)
	assert.Equal(t, user.ID, result.ID)
	assert.True(t, repo.deleted)
	assert.True(t, repo.reset)
}

func TestVerifyTOTPChallengeLocksAfterDurableFailureThreshold(t *testing.T) {
	repo := &fakeTOTPRepo{deleteResult: true, failedAttempts: 10}
	users := new(mockUserRepo)
	svc := newTOTPTestService(repo, users)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	secret := "JBSWY3DPEHPK3PXP"
	user := makeTestUser("alice", "USER", true)
	user.TotpEnabled = true
	user.TotpSecret = &secret
	repo.challenge = db.PltTotpChallenge{TokenHash: hashToken("pt_valid"), UserID: user.ID, AttemptCount: 1}
	users.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	users.On("LockUserUntil", mock.Anything, user.ID, now.Add(15*time.Minute)).Return(nil)

	_, err := svc.VerifyChallenge(context.Background(), "pt_valid", invalidTOTPCode(t, secret, now))

	assert.ErrorIs(t, err, ErrTOTPAccountLocked)
	assert.True(t, repo.deleted)
	users.AssertExpectations(t)
}

func TestAuthenticateReturnsTOTPChallengeAfterPassword(t *testing.T) {
	users := new(mockUserRepo)
	sessions := new(mockSessionRepo)
	audit := new(mockAuditRepo)
	encoder := new(mockPasswordEncoder)
	totpRepo := new(fakeTOTPRepo)
	totpSvc := newTOTPTestService(totpRepo, users)
	svc := NewSecurityService(SecurityDependencies{
		UserRepository:    users,
		PasswordEncoder:   encoder,
		SessionRepository: sessions,
		AuditRepository:   audit,
		TOTPService:       totpSvc,
	}, testSecurityConfig())
	user := makeTestUser("alice", "USER", true)
	user.TotpEnabled = true
	user.TotpSecret = func() *string { value := "JBSWY3DPEHPK3PXP"; return &value }()
	users.On("FindByUsername", mock.Anything, "alice").Return(user, nil)
	encoder.On("Matches", "password123", user.PasswordHash).Return(true)

	result, err := svc.Authenticate(context.Background(), "alice", "password123")

	require.NoError(t, err)
	required, ok := result.(model.TOTPRequired)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(required.PartialToken, "pt_"))
	assert.Equal(t, hashToken(required.PartialToken), totpRepo.challengeHash)
	assert.WithinDuration(t, time.Now().Add(totpChallengeLifetime), totpRepo.challengeExpires, 2*time.Second)
}

func TestVerifyTOTPChallengeRejectsExpiredToken(t *testing.T) {
	svc := newTOTPTestService(new(fakeTOTPRepo), new(mockUserRepo))

	_, err := svc.VerifyChallenge(context.Background(), "pt_missing", "123456")

	assert.ErrorIs(t, err, ErrTOTPChallengeExpired)
}

func TestTOTPUserConversionCarriesEnrollmentState(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	backup := "[]"
	row := makeTestUser("alice", "USER", true)
	row.TotpSecret = &secret
	row.TotpEnabled = true
	row.TotpBackupCodes = &backup
	row.FailedTotpAttempts = 3
	row.LockedUntil = pgtype.Timestamptz{}

	user := security.PltUserToModel(row)

	assert.True(t, user.TOTPEnabled)
	assert.Equal(t, secret, *user.TOTPSecret)
	assert.Equal(t, int32(3), user.FailedTOTPAttempts)
}
