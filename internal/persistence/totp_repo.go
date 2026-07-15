package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type totpRepo struct {
	q *db.Queries
}

func NewTOTPRepository(pool *pgxpool.Pool) TOTPRepository {
	return &totpRepo{q: db.New(pool)}
}

func (r *totpRepo) CreateChallenge(ctx context.Context, tokenHash string, userID uuid.UUID, expiresAt time.Time) error {
	return r.q.CreateTOTPChallenge(ctx, db.CreateTOTPChallengeParams{
		TokenHash: tokenHash,
		UserID:    userID,
		ExpiresAt: expiresAt,
	})
}

func (r *totpRepo) TakeChallengeAttempt(ctx context.Context, tokenHash string, maxAttempts int32) (db.PltTotpChallenge, error) {
	return r.q.TakeTOTPChallengeAttempt(ctx, db.TakeTOTPChallengeAttemptParams{
		TokenHash:    tokenHash,
		AttemptCount: maxAttempts,
	})
}

func (r *totpRepo) DeleteChallenge(ctx context.Context, tokenHash string) (bool, error) {
	count, err := r.q.DeleteTOTPChallenge(ctx, tokenHash)
	return count == 1, err
}

func (r *totpRepo) DeleteExpiredChallenges(ctx context.Context) (int64, error) {
	return r.q.DeleteExpiredTOTPChallenges(ctx)
}

func (r *totpRepo) Enable(ctx context.Context, userID uuid.UUID, secret, backupCodes string) error {
	return r.q.EnableUserTOTP(ctx, db.EnableUserTOTPParams{
		ID:              userID,
		TotpSecret:      &secret,
		TotpBackupCodes: &backupCodes,
	})
}

func (r *totpRepo) Disable(ctx context.Context, userID uuid.UUID) error {
	return r.q.DisableUserTOTP(ctx, userID)
}

func (r *totpRepo) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) (int32, error) {
	return r.q.IncrementFailedTOTPAttempts(ctx, userID)
}

func (r *totpRepo) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	return r.q.ResetFailedTOTPAttempts(ctx, userID)
}

func (r *totpRepo) ReplaceBackupCodes(ctx context.Context, userID uuid.UUID, expected string, replacement *string) (bool, error) {
	count, err := r.q.ReplaceTOTPBackupCodes(ctx, db.ReplaceTOTPBackupCodesParams{
		NewCodes:      replacement,
		UserID:        userID,
		ExpectedCodes: &expected,
	})
	return count == 1, err
}
