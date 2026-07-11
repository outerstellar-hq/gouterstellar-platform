package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type outboxRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) OutboxRepository {
	return &outboxRepo{q: db.New(pool), pool: pool}
}

func (r *outboxRepo) SaveOutbox(ctx context.Context, id uuid.UUID, payloadType, payload, status string) error {
	return r.q.SaveOutbox(ctx, db.SaveOutboxParams{
		ID:          id,
		PayloadType: payloadType,
		Payload:     payload,
		Status:      status,
	})
}

func (r *outboxRepo) ListPending(ctx context.Context, limit int32) ([]db.ListPendingOutboxRow, error) {
	return r.q.ListPendingOutbox(ctx, limit)
}

func (r *outboxRepo) MarkProcessed(ctx context.Context, id uuid.UUID) (db.MarkOutboxProcessedRow, error) {
	return r.q.MarkOutboxProcessed(ctx, id)
}

func (r *outboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, lastError *string) (db.MarkOutboxFailedRow, error) {
	return r.q.MarkOutboxFailed(ctx, db.MarkOutboxFailedParams{
		ID:        id,
		LastError: lastError,
	})
}

func (r *outboxRepo) GetStats(ctx context.Context) (db.GetOutboxStatsRow, error) {
	return r.q.GetOutboxStats(ctx)
}

func (r *outboxRepo) ListFailed(ctx context.Context, limit int32) ([]db.ListFailedOutboxRow, error) {
	return r.q.ListFailedOutbox(ctx, limit)
}

func (r *outboxRepo) ClaimPending(ctx context.Context, limit int32) ([]db.ClaimPendingOutboxRow, error) {
	return r.q.ClaimPendingOutbox(ctx, limit)
}

func (r *outboxRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string, lastError *string) (db.UpdateOutboxStatusRow, error) {
	return r.q.UpdateOutboxStatus(ctx, db.UpdateOutboxStatusParams{
		ID:        id,
		Status:    status,
		LastError: lastError,
	})
}

func (r *outboxRepo) ListDeadLetter(ctx context.Context, limit int32) ([]db.ListDeadLetterOutboxRow, error) {
	return r.q.ListDeadLetterOutbox(ctx, limit)
}
