package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
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

func (r *oauthRepo) CreateUserAndConnection(
	ctx context.Context,
	userID uuid.UUID,
	username, userEmail, passwordHash, provider, subject string,
	oauthEmail *string,
) (db.PltUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return db.PltUser{}, fmt.Errorf("begin OAuth user creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		ID:           userID,
		Username:     username,
		Email:        userEmail,
		PasswordHash: passwordHash,
		Role:         "USER",
		Enabled:      true,
	})
	if err != nil {
		return db.PltUser{}, fmt.Errorf("create OAuth user: %w", err)
	}
	if _, err := q.SaveOAuthConnection(ctx, db.SaveOAuthConnectionParams{
		UserID: userID, Provider: provider, Subject: subject, Email: oauthEmail,
	}); err != nil {
		return db.PltUser{}, fmt.Errorf("save OAuth connection: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.PltUser{}, fmt.Errorf("commit OAuth user creation: %w", err)
	}
	return user, nil
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
