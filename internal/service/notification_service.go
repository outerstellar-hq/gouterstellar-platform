package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type NotificationService struct {
	repo persistence.NotificationRepository
}

func NewNotificationService(repo persistence.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) Create(ctx context.Context, userID uuid.UUID, title, body, nType string) error {
	id := uuid.New()
	_, err := s.repo.SaveNotification(ctx, id, userID, title, body, nType)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (s *NotificationService) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]model.Notification, error) {
	notifications, err := s.repo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	result := make([]model.Notification, len(notifications))
	for i, n := range notifications {
		result[i] = pltNotificationToModel(n)
	}
	return result, nil
}

func (s *NotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *NotificationService) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	_, err := s.repo.MarkRead(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := s.repo.MarkAllRead(ctx, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

func (s *NotificationService) Delete(ctx context.Context, id, userID uuid.UUID) error {
	_, err := s.repo.DeleteNotification(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}

func pltNotificationToModel(n db.PltNotification) model.Notification {
	var readAt *time.Time
	if n.ReadAt.Valid {
		readAt = &n.ReadAt.Time
	}
	return model.Notification{
		ID:        n.ID,
		UserID:    n.UserID,
		Title:     n.Title,
		Body:      n.Body,
		Type:      n.Type,
		ReadAt:    readAt,
		CreatedAt: n.CreatedAt.Time,
	}
}
