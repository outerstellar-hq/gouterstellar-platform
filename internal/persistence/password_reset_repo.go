package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
)

type passwordResetRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) PasswordResetRepository {
	return &passwordResetRepo{q: db.New(pool), pool: pool}
}

func (r *passwordResetRepo) ReplacePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset token replacement: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)
	if err := q.InvalidatePasswordResetTokensForUser(ctx, userID); err != nil {
		return fmt.Errorf("invalidate previous password reset tokens: %w", err)
	}
	if _, err := q.SavePasswordResetToken(ctx, db.SavePasswordResetTokenParams{
		UserID:    userID,
		Token:     tokenHash,
		ExpiresAt: pgtype.Timestamp{Time: expiresAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("save password reset token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset token replacement: %w", err)
	}
	return nil
}

func (r *passwordResetRepo) ConsumePasswordResetToken(ctx context.Context, tokenHash, passwordHash string) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin password reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)
	userID, err := q.ClaimPasswordResetToken(ctx, tokenHash)
	if err != nil {
		return uuid.Nil, err
	}
	if err := q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: userID, PasswordHash: passwordHash}); err != nil {
		return uuid.Nil, fmt.Errorf("update password: %w", err)
	}
	if err := q.DeleteSessionsByUserID(ctx, userID); err != nil {
		return uuid.Nil, fmt.Errorf("revoke sessions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit password reset: %w", err)
	}
	return userID, nil
}
