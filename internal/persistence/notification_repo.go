package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
)

type notificationRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &notificationRepo{q: db.New(pool), pool: pool}
}

func (r *notificationRepo) SaveNotification(ctx context.Context, id, userID uuid.UUID, title, body, notificationType string) (db.PltNotification, error) {
	return r.q.SaveNotification(ctx, db.SaveNotificationParams{
		ID:     id,
		UserID: userID,
		Title:  title,
		Body:   body,
		Type:   notificationType,
	})
}

func (r *notificationRepo) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.PltNotification, error) {
	return r.q.FindNotificationsByUserID(ctx, db.FindNotificationsByUserIDParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *notificationRepo) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.CountUnreadNotifications(ctx, userID)
}

func (r *notificationRepo) MarkRead(ctx context.Context, id, userID uuid.UUID) (db.PltNotification, error) {
	return r.q.MarkNotificationRead(ctx, db.MarkNotificationReadParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *notificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.MarkAllNotificationsRead(ctx, userID)
}

func (r *notificationRepo) DeleteNotification(ctx context.Context, id, userID uuid.UUID) (int64, error) {
	return r.q.DeleteNotification(ctx, db.DeleteNotificationParams{
		ID:     id,
		UserID: userID,
	})
}
