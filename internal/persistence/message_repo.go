package persistence

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type messageRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) MessageRepository {
	return &messageRepo{q: db.New(pool), pool: pool}
}

func (r *messageRepo) ListMessages(ctx context.Context, limit, offset int32) ([]db.PltMessage, error) {
	return r.q.ListMessages(ctx, db.ListMessagesParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *messageRepo) CountMessages(ctx context.Context) (int64, error) {
	return r.q.CountMessages(ctx)
}

func (r *messageRepo) FindBySyncID(ctx context.Context, syncID string) (db.PltMessage, error) {
	return r.q.FindBySyncID(ctx, syncID)
}

func (r *messageRepo) CreateServerMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64) (db.PltMessage, error) {
	return r.q.CreateServerMessage(ctx, db.CreateServerMessageParams{
		SyncID:           syncID,
		Author:           author,
		Content:          content,
		UpdatedAtEpochMs: updatedAtEpochMs,
	})
}

func (r *messageRepo) CreateLocalMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64) (db.PltMessage, error) {
	return r.q.CreateLocalMessage(ctx, db.CreateLocalMessageParams{
		SyncID:           syncID,
		Author:           author,
		Content:          content,
		UpdatedAtEpochMs: updatedAtEpochMs,
	})
}

func (r *messageRepo) UpsertSyncedMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64, deleted bool) (db.PltMessage, error) {
	return r.q.UpsertSyncedMessage(ctx, db.UpsertSyncedMessageParams{
		SyncID:           syncID,
		Author:           author,
		Content:          content,
		UpdatedAtEpochMs: updatedAtEpochMs,
		Deleted:          deleted,
	})
}

func (r *messageRepo) FindChangesSince(ctx context.Context, since int64) ([]db.PltMessage, error) {
	return r.q.FindChangesSince(ctx, since)
}

func (r *messageRepo) ListDirtyMessages(ctx context.Context) ([]db.PltMessage, error) {
	return r.q.ListDirtyMessages(ctx)
}

func (r *messageRepo) CountDirtyMessages(ctx context.Context) (int64, error) {
	return r.q.CountDirtyMessages(ctx)
}

func (r *messageRepo) SoftDeleteMessage(ctx context.Context, syncID string) (db.PltMessage, error) {
	return r.q.SoftDeleteMessage(ctx, syncID)
}

func (r *messageRepo) RestoreMessage(ctx context.Context, syncID string) (db.PltMessage, error) {
	return r.q.RestoreMessage(ctx, syncID)
}

func (r *messageRepo) UpdateMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64, dirty bool, version int64) (db.PltMessage, error) {
	return r.q.UpdateMessage(ctx, db.UpdateMessageParams{
		SyncID:           syncID,
		Author:           author,
		Content:          content,
		UpdatedAtEpochMs: updatedAtEpochMs,
		Dirty:            dirty,
		Version:          version,
	})
}

func (r *messageRepo) MarkConflictMessage(ctx context.Context, syncID string, conflict string) (db.PltMessage, error) {
	return r.q.MarkConflictMessage(ctx, db.MarkConflictMessageParams{
		SyncID:       syncID,
		SyncConflict: &conflict,
	})
}

func (r *messageRepo) ResolveConflictMessage(ctx context.Context, syncID string) (db.PltMessage, error) {
	return r.q.ResolveConflictMessage(ctx, syncID)
}

func (r *messageRepo) MarkCleanMessages(ctx context.Context) error {
	return r.q.MarkCleanMessages(ctx)
}
