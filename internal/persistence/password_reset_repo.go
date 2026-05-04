package persistence

import (
	"context"
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
		Token:     token,
		ExpiresAt: pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
}

func (r *passwordResetRepo) FindByToken(ctx context.Context, token string) (db.PltPasswordResetToken, error) {
	return r.q.FindPasswordResetByToken(ctx, token)
}

func (r *passwordResetRepo) MarkUsed(ctx context.Context, token string) (db.PltPasswordResetToken, error) {
	return r.q.MarkPasswordResetUsed(ctx, token)
}
