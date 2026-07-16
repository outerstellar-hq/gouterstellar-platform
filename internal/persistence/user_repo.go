package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type userRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepo{q: db.New(pool), pool: pool}
}

func (r *userRepo) FindByID(ctx context.Context, id uuid.UUID) (db.PltUser, error) {
	return r.q.FindUserByID(ctx, id)
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (db.PltUser, error) {
	return r.q.FindUserByUsername(ctx, username)
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (db.PltUser, error) {
	return r.q.FindUserByEmail(ctx, email)
}

func (r *userRepo) CreateUser(ctx context.Context, id uuid.UUID, username, email, passwordHash string, role string, enabled bool) (db.PltUser, error) {
	return r.q.CreateUser(ctx, db.CreateUserParams{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Enabled:      enabled,
	})
}

func (r *userRepo) FindAll(ctx context.Context) ([]db.PltUser, error) {
	return r.q.FindAllUsers(ctx)
}

func (r *userRepo) FindPage(ctx context.Context, limit, offset int32) ([]db.PltUser, error) {
	return r.q.FindUserPage(ctx, db.FindUserPageParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *userRepo) CountAll(ctx context.Context) (int64, error) {
	return r.q.CountAllUsers(ctx)
}

func (r *userRepo) CountByRole(ctx context.Context, role string) (int64, error) {
	return r.q.CountUsersByRole(ctx, role)
}

func (r *userRepo) UpdateRole(ctx context.Context, id uuid.UUID, role string) (db.PltUser, error) {
	return r.q.UpdateUserRole(ctx, db.UpdateUserRoleParams{
		ID:   id,
		Role: role,
	})
}

func (r *userRepo) UpdateEnabled(ctx context.Context, id uuid.UUID, enabled bool) (db.PltUser, error) {
	return r.q.UpdateUserEnabled(ctx, db.UpdateUserEnabledParams{
		ID:      id,
		Enabled: enabled,
	})
}

func (r *userRepo) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return r.q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{
		ID:           userID,
		PasswordHash: passwordHash,
	})
}

func (r *userRepo) UpdateLastActivity(ctx context.Context, id uuid.UUID) error {
	return r.q.UpdateLastActivity(ctx, id)
}

func (r *userRepo) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteUserByID(ctx, id)
}

func (r *userRepo) UpdateUsername(ctx context.Context, id uuid.UUID, username string) (db.PltUser, error) {
	return r.q.UpdateUsername(ctx, db.UpdateUsernameParams{
		ID:       id,
		Username: username,
	})
}

func (r *userRepo) UpdateEmail(ctx context.Context, id uuid.UUID, email string) (db.PltUser, error) {
	return r.q.UpdateEmail(ctx, db.UpdateEmailParams{ID: id, Email: email})
}

func (r *userRepo) UpdateAvatarURL(ctx context.Context, id uuid.UUID, avatarURL *string) (db.PltUser, error) {
	return r.q.UpdateAvatarURL(ctx, db.UpdateAvatarURLParams{
		ID:        id,
		AvatarUrl: avatarURL,
	})
}

func (r *userRepo) UpdateNotificationPreferences(ctx context.Context, id uuid.UUID, emailEnabled, pushEnabled bool) (db.PltUser, error) {
	return r.q.UpdateNotificationPreferences(ctx, db.UpdateNotificationPreferencesParams{
		ID:                        id,
		EmailNotificationsEnabled: emailEnabled,
		PushNotificationsEnabled:  pushEnabled,
	})
}

func (r *userRepo) UpdatePreferences(ctx context.Context, id uuid.UUID, language, theme, layout *string) (db.PltUser, error) {
	return r.q.UpdatePreferences(ctx, db.UpdatePreferencesParams{
		ID:       id,
		Language: language,
		Theme:    theme,
		Layout:   layout,
	})
}

func (r *userRepo) SeedAdminUser(ctx context.Context, id uuid.UUID, username, email, passwordHash string) (db.PltUser, error) {
	return r.q.SeedAdminUser(ctx, db.SeedAdminUserParams{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
}

func (r *userRepo) IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) (int32, error) {
	return r.q.IncrementFailedLoginAttempts(ctx, id)
}

func (r *userRepo) ResetLoginFailures(ctx context.Context, id uuid.UUID) error {
	return r.q.ResetLoginFailures(ctx, id)
}

func (r *userRepo) LockUserUntil(ctx context.Context, id uuid.UUID, until time.Time) error {
	return r.q.LockUserUntil(ctx, db.LockUserUntilParams{
		ID:          id,
		LockedUntil: pgtype.Timestamptz{Time: until, Valid: true},
	})
}
