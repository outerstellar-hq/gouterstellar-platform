package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type apiKeyRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewApiKeyRepository(pool *pgxpool.Pool) ApiKeyRepository {
	return &apiKeyRepo{q: db.New(pool), pool: pool}
}

func (r *apiKeyRepo) CreateApiKey(ctx context.Context, userID uuid.UUID, keyHash, keyPrefix, name string) (db.PltApiKey, error) {
	return r.q.CreateApiKey(ctx, db.CreateApiKeyParams{
		UserID:    userID,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Name:      name,
	})
}

func (r *apiKeyRepo) FindByKeyHash(ctx context.Context, keyHash string) (db.PltApiKey, error) {
	return r.q.FindApiKeyByHash(ctx, keyHash)
}

func (r *apiKeyRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]db.PltApiKey, error) {
	return r.q.FindApiKeysByUserID(ctx, userID)
}

func (r *apiKeyRepo) DeleteApiKey(ctx context.Context, id int64, userID uuid.UUID) (int64, error) {
	return r.q.DeleteApiKey(ctx, db.DeleteApiKeyParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *apiKeyRepo) UpdateLastUsed(ctx context.Context, id int64) error {
	return r.q.UpdateApiKeyLastUsed(ctx, id)
}
