package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// WithTx returns a copy of this repository whose underlying sqlc Queries is
// bound to the given transaction. Operations on the returned repository
// participate in the transaction and only persist when the transaction commits.
func (r *outboxRepo) WithTx(tx pgx.Tx) OutboxRepository {
	return &outboxRepo{q: r.q.WithTx(tx), pool: nil}
}

// SaveOutboxTx inserts an outbox row using a caller-supplied transaction. Use
// this (alongside WithTx) when the outbox insert must commit atomically with a
// domain write. The id, payload_type, payload and status columns are set
// explicitly; created_at defaults to CURRENT_TIMESTAMP in the database.
func (r *outboxRepo) SaveOutboxTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, payloadType, payload, status string) error {
	const q = `INSERT INTO plt_outbox (id, payload_type, payload, status) VALUES ($1, $2, $3, $4)`
	_, err := tx.Exec(ctx, q, id, payloadType, payload, status)
	if err != nil {
		return fmt.Errorf("insert outbox entry in tx: %w", err)
	}
	return nil
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
