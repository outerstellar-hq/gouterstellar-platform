package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/security"
)

const (
	totpIssuer               = "Outerstellar"
	totpChallengeLifetime    = 5 * time.Minute
	totpChallengeMaxAttempts = int32(5)
	totpBackupCodeCount      = 16
	totpBackupCodeBytes      = 10
	totpQRSize               = 200
)

var (
	ErrTOTPChallengeExpired = errors.New("totp challenge expired")
	ErrTOTPInvalidCode      = errors.New("invalid totp code")
	ErrTOTPAccountLocked    = errors.New("account locked")
	ErrInvalidPassword      = errors.New("invalid password")
)

type TOTPSetup struct {
	Secret    string
	QRDataURI string
}

type TOTPService struct {
	repo            persistence.TOTPRepository
	userRepo        persistence.UserRepository
	passwordEncoder security.PasswordEncoder
	auditor         Auditor
	config          SecurityConfig
	now             func() time.Time
}

func NewTOTPService(
	repo persistence.TOTPRepository,
	userRepo persistence.UserRepository,
	passwordEncoder security.PasswordEncoder,
	auditor Auditor,
	config SecurityConfig,
) *TOTPService {
	return &TOTPService{
		repo:            repo,
		userRepo:        userRepo,
		passwordEncoder: passwordEncoder,
		auditor:         auditor,
		config:          config,
		now:             time.Now,
	}
}

func (s *TOTPService) CreateChallenge(ctx context.Context, userID uuid.UUID) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate TOTP challenge: %w", err)
	}
	token := "pt_" + hex.EncodeToString(raw)
	if err := s.repo.CreateChallenge(ctx, hashToken(token), userID, s.now().Add(totpChallengeLifetime)); err != nil {
		return "", fmt.Errorf("store TOTP challenge: %w", err)
	}
	return token, nil
}

func (s *TOTPService) GenerateSetup(accountName string) (TOTPSetup, error) {
	return s.generateSetup(accountName, nil)
}

func (s *TOTPService) RestoreSetup(accountName, secret string) (TOTPSetup, error) {
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return TOTPSetup{}, fmt.Errorf("decode TOTP secret: %w", err)
	}
	return s.generateSetup(accountName, decoded)
}

func (s *TOTPService) generateSetup(accountName string, secret []byte) (TOTPSetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
		Period:      30,
		SecretSize:  32,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
		Secret:      secret,
	})
	if err != nil {
		return TOTPSetup{}, fmt.Errorf("generate TOTP secret: %w", err)
	}
	image, err := key.Image(totpQRSize, totpQRSize)
	if err != nil {
		return TOTPSetup{}, fmt.Errorf("generate TOTP QR code: %w", err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image); err != nil {
		return TOTPSetup{}, fmt.Errorf("encode TOTP QR code: %w", err)
	}
	return TOTPSetup{
		Secret:    key.Secret(),
		QRDataURI: "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()),
	}, nil
}

func (s *TOTPService) ConfirmEnrollment(ctx context.Context, userID uuid.UUID, secret, code string) ([]string, error) {
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret)); err != nil {
		return nil, ErrTOTPInvalidCode
	}
	valid, err := s.verifyCode(secret, code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrTOTPInvalidCode
	}
	rawCodes, encodedCodes, err := s.generateBackupCodes()
	if err != nil {
		return nil, err
	}
	if err := s.repo.Enable(ctx, userID, secret, encodedCodes); err != nil {
		return nil, fmt.Errorf("enable TOTP: %w", err)
	}
	s.record(ctx, "TOTP_ENABLED", userID, "Two-factor authentication enabled")
	return rawCodes, nil
}

func (s *TOTPService) Disable(ctx context.Context, userID uuid.UUID, password string) error {
	row, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return ErrInvalidPassword
	}
	user := security.PltUserToModel(row)
	if !s.passwordEncoder.Matches(password, user.PasswordHash) {
		return ErrInvalidPassword
	}
	if err := s.repo.Disable(ctx, userID); err != nil {
		return fmt.Errorf("disable TOTP: %w", err)
	}
	s.record(ctx, "TOTP_DISABLED", userID, "Two-factor authentication disabled")
	return nil
}

func (s *TOTPService) VerifyChallenge(ctx context.Context, partialToken, code string) (*model.User, error) {
	challenge, err := s.repo.TakeChallengeAttempt(ctx, hashToken(partialToken), totpChallengeMaxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTOTPChallengeExpired
	}
	if err != nil {
		return nil, fmt.Errorf("take TOTP challenge attempt: %w", err)
	}
	row, err := s.userRepo.FindByID(ctx, challenge.UserID)
	if err != nil {
		return nil, ErrTOTPChallengeExpired
	}
	user := security.PltUserToModel(row)
	if !user.Enabled || !user.TOTPEnabled || user.TOTPSecret == nil {
		return nil, ErrTOTPChallengeExpired
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		return nil, ErrTOTPAccountLocked
	}

	valid, verifyErr := s.verifyCode(*user.TOTPSecret, code)
	if verifyErr != nil {
		return nil, verifyErr
	}
	if valid {
		return s.completeChallenge(ctx, partialToken, user)
	}
	if user.TOTPBackupCodes != nil {
		matched, replacement, backupErr := s.consumeBackupCode(code, *user.TOTPBackupCodes)
		if backupErr != nil {
			return nil, backupErr
		}
		if matched {
			replaced, replaceErr := s.repo.ReplaceBackupCodes(ctx, user.ID, *user.TOTPBackupCodes, replacement)
			if replaceErr != nil {
				return nil, fmt.Errorf("consume TOTP backup code: %w", replaceErr)
			}
			if !replaced {
				return nil, ErrTOTPInvalidCode
			}
			return s.completeChallenge(ctx, partialToken, user)
		}
	}

	attempts, err := s.repo.IncrementFailedAttempts(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("record failed TOTP attempt: %w", err)
	}
	if attempts >= s.config.MaxFailedLoginAttempts {
		until := s.now().Add(s.config.LockoutDuration)
		if err := s.userRepo.LockUserUntil(ctx, user.ID, until); err != nil {
			return nil, fmt.Errorf("lock account after TOTP failures: %w", err)
		}
		_, _ = s.repo.DeleteChallenge(ctx, hashToken(partialToken))
		return nil, ErrTOTPAccountLocked
	}
	if challenge.AttemptCount >= totpChallengeMaxAttempts {
		_, _ = s.repo.DeleteChallenge(ctx, hashToken(partialToken))
	}
	return nil, ErrTOTPInvalidCode
}

func (s *TOTPService) DeleteExpiredChallenges(ctx context.Context) error {
	if _, err := s.repo.DeleteExpiredChallenges(ctx); err != nil {
		return fmt.Errorf("delete expired TOTP challenges: %w", err)
	}
	return nil
}

func (s *TOTPService) ResetFailures(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.ResetFailedAttempts(ctx, userID); err != nil {
		return fmt.Errorf("reset failed TOTP attempts: %w", err)
	}
	return nil
}

func (s *TOTPService) BackupCodeCount(encoded *string) (int, error) {
	if encoded == nil || *encoded == "" {
		return 0, nil
	}
	var hashes []string
	if err := json.Unmarshal([]byte(*encoded), &hashes); err != nil {
		return 0, fmt.Errorf("decode TOTP backup codes: %w", err)
	}
	return len(hashes), nil
}

func (s *TOTPService) completeChallenge(ctx context.Context, partialToken string, user *model.User) (*model.User, error) {
	deleted, err := s.repo.DeleteChallenge(ctx, hashToken(partialToken))
	if err != nil {
		return nil, fmt.Errorf("consume TOTP challenge: %w", err)
	}
	if !deleted {
		return nil, ErrTOTPChallengeExpired
	}
	if err := s.repo.ResetFailedAttempts(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("reset failed TOTP attempts: %w", err)
	}
	if err := s.userRepo.UpdateLastActivity(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("update activity after TOTP verification: %w", err)
	}
	if s.auditor != nil {
		s.auditor.Record(ctx, "USER_LOGIN", &user.ID, &user.Username, nil, nil, "Login successful with TOTP")
	}
	return user, nil
}

func (s *TOTPService) verifyCode(secret, code string) (bool, error) {
	if len(code) != 6 {
		return false, nil
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false, nil
		}
	}
	valid, err := totp.ValidateCustom(code, secret, s.now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false, fmt.Errorf("validate TOTP code: %w", err)
	}
	return valid, nil
}

func (s *TOTPService) generateBackupCodes() ([]string, string, error) {
	rawCodes := make([]string, totpBackupCodeCount)
	hashedCodes := make([]string, totpBackupCodeCount)
	for i := range rawCodes {
		raw := make([]byte, totpBackupCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, "", fmt.Errorf("generate TOTP backup code: %w", err)
		}
		plain := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		rawCodes[i] = plain[:8] + "-" + plain[8:]
		hash, err := s.passwordEncoder.Encode(rawCodes[i])
		if err != nil {
			return nil, "", fmt.Errorf("hash TOTP backup code: %w", err)
		}
		hashedCodes[i] = hash
	}
	encoded, err := json.Marshal(hashedCodes)
	if err != nil {
		return nil, "", fmt.Errorf("encode TOTP backup codes: %w", err)
	}
	return rawCodes, string(encoded), nil
}

func (s *TOTPService) consumeBackupCode(code, encoded string) (bool, *string, error) {
	var hashes []string
	if err := json.Unmarshal([]byte(encoded), &hashes); err != nil {
		return false, nil, fmt.Errorf("decode TOTP backup codes: %w", err)
	}
	for i, hash := range hashes {
		if !s.passwordEncoder.Matches(strings.ToUpper(code), hash) {
			continue
		}
		hashes = append(hashes[:i], hashes[i+1:]...)
		if len(hashes) == 0 {
			return true, nil, nil
		}
		encodedReplacement, err := json.Marshal(hashes)
		if err != nil {
			return false, nil, fmt.Errorf("encode remaining TOTP backup codes: %w", err)
		}
		replacement := string(encodedReplacement)
		return true, &replacement, nil
	}
	return false, nil, nil
}

func (s *TOTPService) record(ctx context.Context, action string, userID uuid.UUID, detail string) {
	if s.auditor != nil {
		s.auditor.Record(ctx, action, &userID, nil, &userID, nil, detail)
	}
}
