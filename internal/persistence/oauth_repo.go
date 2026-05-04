package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type oauthRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewOAuthRepository(pool *pgxpool.Pool) OAuthRepository {
	return &oauthRepo{q: db.New(pool), pool: pool}
}

func (r *oauthRepo) FindByProviderSubject(ctx context.Context, provider, subject string) (db.PltOauthConnection, error) {
	return r.q.FindOAuthByProviderSubject(ctx, db.FindOAuthByProviderSubjectParams{
		Provider: provider,
		Subject:  subject,
	})
}

func (r *oauthRepo) SaveOAuthConnection(ctx context.Context, userID uuid.UUID, provider, subject string, email *string) (db.PltOauthConnection, error) {
	return r.q.SaveOAuthConnection(ctx, db.SaveOAuthConnectionParams{
		UserID:   userID,
		Provider: provider,
		Subject:  subject,
		Email:    email,
	})
}

func (r *oauthRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]db.PltOauthConnection, error) {
	return r.q.FindOAuthByUserID(ctx, userID)
}

func (r *oauthRepo) DeleteOAuthConnection(ctx context.Context, id int64, userID uuid.UUID) (int64, error) {
	return r.q.DeleteOAuthConnection(ctx, db.DeleteOAuthConnectionParams{
		ID:     id,
		UserID: userID,
	})
}
