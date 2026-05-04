package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type auditRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) AuditRepository {
	return &auditRepo{q: db.New(pool), pool: pool}
}

func (r *auditRepo) LogAudit(ctx context.Context, actorID *uuid.UUID, actorUsername *string, targetID *uuid.UUID, targetUsername *string, action, detail string) (db.PltAuditLog, error) {
	return r.q.LogAudit(ctx, db.LogAuditParams{
		ActorID:        uuidPtrToPgtype(actorID),
		ActorUsername:  actorUsername,
		TargetID:       uuidPtrToPgtype(targetID),
		TargetUsername: targetUsername,
		Action:         action,
		Detail:         &detail,
	})
}

func (r *auditRepo) FindRecent(ctx context.Context, limit int32) ([]db.PltAuditLog, error) {
	return r.q.FindRecentAudit(ctx, limit)
}

func (r *auditRepo) FindPage(ctx context.Context, limit, offset int32) ([]db.PltAuditLog, error) {
	return r.q.FindAuditPage(ctx, db.FindAuditPageParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *auditRepo) CountAll(ctx context.Context) (int64, error) {
	return r.q.CountAllAudit(ctx)
}

func uuidPtrToPgtype(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}
