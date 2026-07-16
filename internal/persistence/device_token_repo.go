package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type deviceTokenRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewDeviceTokenRepository(pool *pgxpool.Pool) DeviceTokenRepository {
	return &deviceTokenRepo{q: db.New(pool), pool: pool}
}

func (r *deviceTokenRepo) UpsertDeviceToken(ctx context.Context, userID uuid.UUID, platform, token string, appBundle *string) (db.PltDeviceToken, error) {
	return r.q.UpsertDeviceToken(ctx, db.UpsertDeviceTokenParams{
		UserID:    userID,
		Platform:  platform,
		Token:     token,
		AppBundle: appBundle,
	})
}

func (r *deviceTokenRepo) DeleteDeviceToken(ctx context.Context, id int64, userID uuid.UUID) (int64, error) {
	return r.q.DeleteDeviceToken(ctx, db.DeleteDeviceTokenParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *deviceTokenRepo) DeleteDeviceTokenByValue(ctx context.Context, token string, userID uuid.UUID) (int64, error) {
	return r.q.DeleteDeviceTokenByValue(ctx, db.DeleteDeviceTokenByValueParams{
		Token:  token,
		UserID: userID,
	})
}

func (r *deviceTokenRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]db.PltDeviceToken, error) {
	return r.q.FindDeviceTokensByUserID(ctx, userID)
}

func (r *deviceTokenRepo) DeleteAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.DeleteAllDeviceTokensForUser(ctx, userID)
}
