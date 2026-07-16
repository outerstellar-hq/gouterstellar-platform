package persistence

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
)

type syncStateRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewSyncStateRepository(pool *pgxpool.Pool) *syncStateRepo {
	return &syncStateRepo{q: db.New(pool), pool: pool}
}

func (r *syncStateRepo) GetSyncState(ctx context.Context, stateKey string) (db.PltSyncState, error) {
	return r.q.GetSyncState(ctx, stateKey)
}

func (r *syncStateRepo) SetSyncState(ctx context.Context, stateKey string, stateValue int64) error {
	return r.q.SetSyncState(ctx, db.SetSyncStateParams{
		StateKey:   stateKey,
		StateValue: stateValue,
	})
}

func (r *syncStateRepo) UpsertSyncState(ctx context.Context, stateKey string, stateValue int64) (db.PltSyncState, error) {
	return r.q.UpsertSyncState(ctx, db.UpsertSyncStateParams{
		StateKey:   stateKey,
		StateValue: stateValue,
	})
}
