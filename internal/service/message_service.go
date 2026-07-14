package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type MessageService struct {
	repo                persistence.MessageRepository
	outbox              persistence.OutboxRepository
	txMgr               TransactionRunner
	cache               *persistence.MessageCache
	eventPub            EventPublisher
	notificationService *NotificationService
	emailService        EmailService
	pipeline            *WritePipeline
}

func NewMessageService(
	repo persistence.MessageRepository,
	outbox persistence.OutboxRepository,
	txMgr TransactionRunner,
	cache *persistence.MessageCache,
	eventPub EventPublisher,
	notificationService *NotificationService,
	emailService EmailService,
) *MessageService {
	s := &MessageService{
		repo:                repo,
		outbox:              outbox,
		txMgr:               txMgr,
		cache:               cache,
		eventPub:            eventPub,
		notificationService: notificationService,
		emailService:        emailService,
	}
	// The pipeline reuses the service's already-wired cache/event/notifier/email
	// dependencies so side-effects fire through the same instances. Built once
	// at construction so each mutation just calls the relevant hook.
	s.pipeline = NewWritePipeline(cache, eventPub, notificationService, emailService)
	return s
}

func (s *MessageService) ListMessages(ctx context.Context, limit, offset int32) (*model.PagedResult[model.MessageSummary], error) {
	total, err := s.repo.CountMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("count messages: %w", err)
	}

	cacheKey := fmt.Sprintf("messages:list:%d:%d", limit, offset)
	cached := s.cache.GetOrSet(cacheKey, func() interface{} {
		messages, err := s.repo.ListMessages(ctx, limit, offset)
		if err != nil {
			slog.Error("Failed to list messages", "error", err)
			return nil
		}
		summaries := make([]model.MessageSummary, len(messages))
		for i, m := range messages {
			summaries[i] = pltMessageToSummary(m)
		}
		return summaries
	})

	if cached == nil {
		return &model.PagedResult[model.MessageSummary]{
			Items:    []model.MessageSummary{},
			Metadata: model.NewPaginationMetadata(1, int(limit), 0),
		}, nil
	}

	summaries := cached.([]model.MessageSummary)
	page := int(offset)/int(limit) + 1
	return &model.PagedResult[model.MessageSummary]{
		Items:    summaries,
		Metadata: model.NewPaginationMetadata(page, int(limit), total),
	}, nil
}

// SearchMessages returns a page of non-deleted messages whose content or author
// match the given query (case-insensitive ILIKE). Search results are not cached
// because they are highly variable and rarely re-requested with the same terms.
func (s *MessageService) SearchMessages(ctx context.Context, query string, limit, offset int32) (*model.PagedResult[model.MessageSummary], error) {
	total, err := s.repo.CountSearchMessages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count search messages: %w", err)
	}

	messages, err := s.repo.SearchMessages(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}

	summaries := make([]model.MessageSummary, len(messages))
	for i, m := range messages {
		summaries[i] = pltMessageToSummary(m)
	}

	page := int(offset)/int(limit) + 1
	return &model.PagedResult[model.MessageSummary]{
		Items:    summaries,
		Metadata: model.NewPaginationMetadata(page, int(limit), total),
	}, nil
}

// ListMessagesByYear returns one page of non-deleted messages whose updated_at
// timestamp falls in the given calendar year. The year filter is evaluated at
// the database level so paging counts stay correct.
func (s *MessageService) ListMessagesByYear(ctx context.Context, year int, limit, offset int32) (*model.PagedResult[model.MessageSummary], error) {
	total, err := s.repo.CountMessagesByYear(ctx, year)
	if err != nil {
		return nil, fmt.Errorf("count messages by year: %w", err)
	}

	messages, err := s.repo.ListMessagesByYear(ctx, year, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list messages by year: %w", err)
	}

	summaries := make([]model.MessageSummary, len(messages))
	for i, m := range messages {
		summaries[i] = pltMessageToSummary(m)
	}

	page := int(offset)/int(limit) + 1
	return &model.PagedResult[model.MessageSummary]{
		Items:    summaries,
		Metadata: model.NewPaginationMetadata(page, int(limit), total),
	}, nil
}

// GetMessageYears returns the distinct calendar years (descending) for which
// non-deleted messages exist. Used to populate the year filter UI. sqlc emits
// []int32 from the EXTRACT expression, so the values are widened to int here to
// match the viewmodel's []int slice.
func (s *MessageService) GetMessageYears(ctx context.Context) ([]int, error) {
	years, err := s.repo.ListMessageYears(ctx)
	if err != nil {
		return nil, fmt.Errorf("list message years: %w", err)
	}
	result := make([]int, len(years))
	for i, y := range years {
		result[i] = int(y)
	}
	return result, nil
}

func (s *MessageService) ListDeletedMessages(ctx context.Context, limit, offset int32) (*model.PagedResult[model.MessageSummary], error) {
	messages, err := s.repo.ListMessages(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	var deleted []model.MessageSummary
	for _, m := range messages {
		if m.Deleted {
			deleted = append(deleted, pltMessageToSummary(m))
		}
	}

	if deleted == nil {
		deleted = []model.MessageSummary{}
	}

	return &model.PagedResult[model.MessageSummary]{
		Items:    deleted,
		Metadata: model.NewPaginationMetadata(int(offset)/int(limit)+1, int(limit), int64(len(deleted))),
	}, nil
}

func (s *MessageService) FindBySyncID(ctx context.Context, syncID string) (*model.StoredMessage, error) {
	cacheKey := "message:" + syncID
	cached := s.cache.GetOrSet(cacheKey, func() interface{} {
		m, err := s.repo.FindBySyncID(ctx, syncID)
		if err != nil {
			return nil
		}
		stored := pltMessageToStored(m)
		return stored
	})

	if cached == nil {
		return nil, &model.MessageNotFoundError{SyncID: syncID}
	}

	return cached.(*model.StoredMessage), nil
}

func (s *MessageService) ListDirtyMessages(ctx context.Context) ([]model.StoredMessage, error) {
	messages, err := s.repo.ListDirtyMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list dirty messages: %w", err)
	}

	result := make([]model.StoredMessage, len(messages))
	for i, m := range messages {
		result[i] = *pltMessageToStored(m)
	}
	return result, nil
}

func (s *MessageService) CreateServerMessage(ctx context.Context, author, content string) (*model.StoredMessage, error) {
	if strings.TrimSpace(author) == "" || strings.TrimSpace(content) == "" {
		return nil, &model.ValidationError{Errors: []string{"Author and content must not be blank"}}
	}

	syncID := "srv_" + uuid.New().String()
	now := time.Now().UnixMilli()

	var m db.PltMessage
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		created, err := s.repo.WithTx(tx).CreateServerMessage(ctx, syncID, author, content, now)
		if err != nil {
			return fmt.Errorf("create server message: %w", err)
		}
		if err := s.saveOutboxEntryTx(ctx, tx, syncID, created); err != nil {
			return err
		}
		m = created
		return nil
	})
	if err != nil {
		return nil, err
	}

	stored := pltMessageToStored(m)
	s.pipeline.AfterMessageCreated(ctx, author, content)

	return stored, nil
}

func (s *MessageService) CreateLocalMessage(ctx context.Context, author, content string) (*model.StoredMessage, error) {
	if strings.TrimSpace(author) == "" || strings.TrimSpace(content) == "" {
		return nil, &model.ValidationError{Errors: []string{"Author and content must not be blank"}}
	}

	syncID := "loc_" + uuid.New().String()
	now := time.Now().UnixMilli()

	var m db.PltMessage
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		created, err := s.repo.WithTx(tx).CreateLocalMessage(ctx, syncID, author, content, now)
		if err != nil {
			return fmt.Errorf("create local message: %w", err)
		}
		if err := s.saveOutboxEntryTx(ctx, tx, syncID, created); err != nil {
			return err
		}
		m = created
		return nil
	})
	if err != nil {
		return nil, err
	}

	stored := pltMessageToStored(m)
	// Local creates don't notify or email — they only refresh the cache and
	// broadcast the change. AfterMessageCreated is overkill here (it would fire
	// notifications), so invalidate the prefix directly. A future hook could
	// model "local create" explicitly if notifications are ever wanted.
	s.cache.InvalidateByPrefix("messages:")
	s.eventPub.PublishRefresh(ActorUserIDFromContext(ctx), "messages")

	return stored, nil
}

func (s *MessageService) GetChangesSince(ctx context.Context, since int64) (*model.SyncPullResponse, error) {
	messages, err := s.repo.FindChangesSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changes since: %w", err)
	}

	syncMessages := make([]model.SyncMessage, len(messages))
	for i, m := range messages {
		syncMessages[i] = pltMessageToSyncMessage(m)
	}

	return &model.SyncPullResponse{
		Messages:        syncMessages,
		ServerTimestamp: time.Now().UnixMilli(),
	}, nil
}

func (s *MessageService) ProcessPushRequest(ctx context.Context, req *model.SyncPushRequest) (*model.SyncPushResponse, error) {
	appliedCount := 0
	var conflicts []model.SyncConflict

	for _, msg := range req.Messages {
		existing, err := s.repo.FindBySyncID(ctx, msg.SyncID)
		if err != nil {
			_, err := s.repo.UpsertSyncedMessage(ctx, msg.SyncID, msg.Author, msg.Content, msg.UpdatedAtEpochMs, msg.Deleted)
			if err != nil {
				conflicts = append(conflicts, model.SyncConflict{
					SyncID:        msg.SyncID,
					Reason:        fmt.Sprintf("Failed to upsert: %v", err),
					ServerMessage: nil,
				})
				continue
			}
			appliedCount++
			continue
		}

		if existing.Version > 0 && existing.UpdatedAtEpochMs > msg.UpdatedAtEpochMs {
			serverMsg := pltMessageToSyncMessage(existing)
			conflicts = append(conflicts, model.SyncConflict{
				SyncID:        msg.SyncID,
				Reason:        "Server has newer version",
				ServerMessage: &serverMsg,
			})
			continue
		}

		_, err = s.repo.UpsertSyncedMessage(ctx, msg.SyncID, msg.Author, msg.Content, msg.UpdatedAtEpochMs, msg.Deleted)
		if err != nil {
			conflicts = append(conflicts, model.SyncConflict{
				SyncID:        msg.SyncID,
				Reason:        fmt.Sprintf("Failed to upsert: %v", err),
				ServerMessage: nil,
			})
			continue
		}
		appliedCount++
	}

	if conflicts == nil {
		conflicts = []model.SyncConflict{}
	}

	s.cache.InvalidateAll()
	s.eventPub.PublishRefresh(ActorUserIDFromContext(ctx), "messages")

	return &model.SyncPushResponse{
		AppliedCount: appliedCount,
		Conflicts:    conflicts,
	}, nil
}

func (s *MessageService) Restore(ctx context.Context, syncID string) error {
	_, err := s.repo.RestoreMessage(ctx, syncID)
	if err != nil {
		return fmt.Errorf("restore message: %w", err)
	}
	s.pipeline.AfterMessageChange(ctx, syncID)
	return nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, syncID string) error {
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		deleted, err := s.repo.WithTx(tx).SoftDeleteMessage(ctx, syncID)
		if err != nil {
			return fmt.Errorf("delete message: %w", err)
		}
		return s.saveOutboxEntryTx(ctx, tx, syncID, deleted)
	})
	if err != nil {
		return err
	}

	s.pipeline.AfterMessageChange(ctx, syncID)
	return nil
}

func (s *MessageService) UpdateMessage(ctx context.Context, msg *model.StoredMessage) (*model.StoredMessage, error) {
	var updated db.PltMessage
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		u, err := s.repo.WithTx(tx).UpdateMessage(ctx, msg.SyncID, msg.Author, msg.Content, msg.UpdatedAtEpochMs, msg.Dirty, msg.Version)
		if err != nil {
			return fmt.Errorf("update message: %w", err)
		}
		if err := s.saveOutboxEntryTx(ctx, tx, msg.SyncID, u); err != nil {
			return err
		}
		updated = u
		return nil
	})
	if err != nil {
		return nil, err
	}

	stored := pltMessageToStored(updated)
	s.pipeline.AfterMessageChange(ctx, msg.SyncID)
	return stored, nil
}

func (s *MessageService) ResolveConflict(ctx context.Context, syncID string, strategy model.ConflictStrategy) error {
	existing, err := s.repo.FindBySyncID(ctx, syncID)
	if err != nil {
		return &model.MessageNotFoundError{SyncID: syncID}
	}

	if existing.SyncConflict == nil {
		return nil
	}

	var serverSyncMsg model.SyncMessage
	if err := json.Unmarshal([]byte(*existing.SyncConflict), &serverSyncMsg); err != nil {
		return fmt.Errorf("parse conflict JSON: %w", err)
	}

	switch strategy {
	case model.ConflictMine:
		_, err = s.repo.ResolveConflictMessage(ctx, syncID)
		if err != nil {
			return fmt.Errorf("resolve conflict: %w", err)
		}
	case model.ConflictServer:
		_, err = s.repo.UpdateMessage(ctx, syncID, serverSyncMsg.Author, serverSyncMsg.Content, serverSyncMsg.UpdatedAtEpochMs, false, existing.Version)
		if err != nil {
			return fmt.Errorf("apply server version: %w", err)
		}
		_, err = s.repo.ResolveConflictMessage(ctx, syncID)
		if err != nil {
			return fmt.Errorf("resolve conflict after server apply: %w", err)
		}
	}

	s.pipeline.AfterMessageChange(ctx, syncID)
	return nil
}

// saveOutboxEntryTx serializes and inserts an outbox entry within a caller-
// supplied transaction. It is the transactional counterpart of saveOutboxEntry
// and is intended for the TODO: transactional outbox write below.
func (s *MessageService) saveOutboxEntryTx(ctx context.Context, tx pgx.Tx, syncID string, m db.PltMessage) error {
	syncMsg := pltMessageToSyncMessage(m)
	payload, err := model.SyncMessageToJSON(syncMsg)
	if err != nil {
		return fmt.Errorf("serialize outbox payload: %w", err)
	}
	return s.outbox.SaveOutboxTx(ctx, tx, uuid.New(), "MESSAGE_SYNC", payload, "PENDING")
}

func pltMessageToStored(m db.PltMessage) *model.StoredMessage {
	return &model.StoredMessage{
		SyncID:           m.SyncID,
		Author:           m.Author,
		Content:          m.Content,
		UpdatedAtEpochMs: m.UpdatedAtEpochMs,
		Dirty:            m.Dirty,
		Deleted:          m.Deleted,
		Version:          m.Version,
		SyncConflict:     m.SyncConflict,
	}
}

func pltMessageToSummary(m db.PltMessage) model.MessageSummary {
	return model.MessageSummary{
		SyncID:           m.SyncID,
		Author:           m.Author,
		Content:          m.Content,
		UpdatedAtEpochMs: m.UpdatedAtEpochMs,
		Dirty:            m.Dirty,
		Version:          m.Version,
		HasConflict:      m.SyncConflict != nil,
	}
}

func pltMessageToSyncMessage(m db.PltMessage) model.SyncMessage {
	return model.SyncMessage{
		SyncID:           m.SyncID,
		Author:           m.Author,
		Content:          m.Content,
		UpdatedAtEpochMs: m.UpdatedAtEpochMs,
		Deleted:          m.Deleted,
	}
}

// CountMessages returns the total number of non-deleted messages.
func (s *MessageService) CountMessages(ctx context.Context) (int64, error) {
	return s.repo.CountMessages(ctx)
}

// notifyActor records a best-effort notification for the user acting on the
// current request. It is nil-safe: if no NotificationService is wired or no
// authenticated user is present in the context, it does nothing. Notification
// failures are logged but never propagated, so a notification hiccup cannot
// fail the originating write.
func (s *MessageService) notifyActor(ctx context.Context, title, body, nType string) {
	if s.notificationService == nil {
		return
	}
	user := UserFromContext(ctx)
	if user == nil {
		return
	}
	if err := s.notificationService.Create(ctx, user.ID, title, body, nType); err != nil {
		slog.Warn("Failed to create notification", "title", title, "error", err)
	}
}

// notifyActorByEmail sends a best-effort notification email to the user acting
// on the current request when they have email notifications enabled and an
// EmailService is wired. It is nil-safe and never propagates errors — email
// delivery is opportunistic and must not fail the originating write.
func (s *MessageService) notifyActorByEmail(ctx context.Context, subject, body string) {
	if s.emailService == nil {
		return
	}
	user := UserFromContext(ctx)
	if user == nil || !user.EmailNotificationsEnabled || user.Email == "" {
		return
	}
	if err := s.emailService.Send(user.Email, subject, body); err != nil {
		slog.Error("Failed to send notification email", "subject", subject, "error", err)
	}
}

// truncateContent caps a string to maxNotificationBodyLen characters, appending
// an ellipsis when truncated, so notification bodies stay readable.
func truncateContent(s string) string {
	const maxNotificationBodyLen = 100
	if len(s) <= maxNotificationBodyLen {
		return s
	}
	return s[:maxNotificationBodyLen] + "…"
}
