package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type passwordResetRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) PasswordResetRepository {
	return &passwordResetRepo{q: db.New(pool), pool: pool}
}

func (r *passwordResetRepo) SavePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (db.PltPasswordResetToken, error) {
	return r.q.SavePasswordResetToken(ctx, db.SavePasswordResetTokenParams{
		UserID:    userID,
		Token:     hashResetToken(token),
		ExpiresAt: pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
}

func (r *passwordResetRepo) Consume(ctx context.Context, token, passwordHash string) (db.PltUser, error) {
	return r.q.ConsumePasswordReset(ctx, db.ConsumePasswordResetParams{
		Token:        hashResetToken(token),
		PasswordHash: passwordHash,
	})
}

func hashResetToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
