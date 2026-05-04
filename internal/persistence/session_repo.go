package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type sessionRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) SessionRepository {
	return &sessionRepo{q: db.New(pool), pool: pool}
}

func (r *sessionRepo) CreateSession(ctx context.Context, tokenHash string, userID uuid.UUID, expiresAt time.Time) (db.PltSession, error) {
	return r.q.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: tokenHash,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
}

func (r *sessionRepo) FindByTokenHash(ctx context.Context, tokenHash string) (db.PltSession, error) {
	return r.q.FindSessionByTokenHash(ctx, tokenHash)
}

func (r *sessionRepo) FindByTokenHashIncludingExpired(ctx context.Context, tokenHash string) (db.PltSession, error) {
	return r.q.FindSessionByTokenHashIncludingExpired(ctx, tokenHash)
}

func (r *sessionRepo) UpdateExpiresAt(ctx context.Context, tokenHash string, expiresAt time.Time) (db.PltSession, error) {
	return r.q.UpdateSessionExpiresAt(ctx, db.UpdateSessionExpiresAtParams{
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
}

func (r *sessionRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	return r.q.DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *sessionRepo) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q.DeleteSessionsByUserID(ctx, userID)
}

func (r *sessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return r.q.DeleteExpiredSessions(ctx)
}
