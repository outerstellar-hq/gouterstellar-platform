package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
)

// maxRetryCount is the threshold at which an outbox entry is moved to the
// DEAD_LETTER status instead of being retried. The ClaimPendingOutbox query
// re-claims PROCESSING rows whose retry_count < 5, so once an entry reaches
// this threshold it will no longer be picked up automatically.
const maxRetryCount int32 = 5

type OutboxProcessor struct {
	outboxRepo persistence.OutboxRepository
	txMgr      *persistence.TransactionManager
	eventPub   EventPublisher
}

func NewOutboxProcessor(
	outboxRepo persistence.OutboxRepository,
	txMgr *persistence.TransactionManager,
	eventPub EventPublisher,
) *OutboxProcessor {
	return &OutboxProcessor{
		outboxRepo: outboxRepo,
		txMgr:      txMgr,
		eventPub:   eventPub,
	}
}

// ProcessPending atomically claims a batch of pending outbox entries using
// FOR UPDATE SKIP LOCKED (so concurrent workers do not duplicate-process),
// processes each, and updates the status:
//   - success                       -> PROCESSED
//   - failure, retry_count < 5      -> PENDING (will be retried)
//   - failure, retry_count >= 5     -> DEAD_LETTER (permanent failure)
func (p *OutboxProcessor) ProcessPending(ctx context.Context) error {
	entries, err := p.outboxRepo.ClaimPending(ctx, 10)
	if err != nil {
		return fmt.Errorf("claim pending outbox: %w", err)
	}

	for _, entry := range entries {
		p.processClaimedEntry(ctx, outboxEntry{
			ID:          entry.ID,
			PayloadType: entry.PayloadType,
			Payload:     entry.Payload,
			RetryCount:  entry.RetryCount,
		})
	}

	return nil
}

func (p *OutboxProcessor) processClaimedEntry(ctx context.Context, entry outboxEntry) {
	if err := p.processEntry(ctx, entry.ID, entry.PayloadType, entry.Payload); err != nil {
		errMsg := err.Error()
		status := p.failureStatus(entry.RetryCount)
		if _, markErr := p.outboxRepo.UpdateStatus(ctx, entry.ID, status, &errMsg); markErr != nil {
			slog.Error("Failed to update outbox entry status after processing failure",
				"id", entry.ID, "status", status, "error", markErr)
		}
		slog.Error("Failed to process outbox entry",
			"id", entry.ID, "retry_count", retryValue(entry.RetryCount), "next_status", status, "error", err)
		return
	}

	if _, err := p.outboxRepo.UpdateStatus(ctx, entry.ID, "PROCESSED", nil); err != nil {
		slog.Error("Failed to mark outbox entry as processed", "id", entry.ID, "error", err)
	}
}

// failureStatus decides whether to retry (PENDING) or dead-letter based on
// the retry_count that was already incremented by ClaimPendingOutbox.
func (p *OutboxProcessor) failureStatus(retryCount *int32) string {
	if retryCount != nil && *retryCount >= maxRetryCount {
		return "DEAD_LETTER"
	}
	return "PENDING"
}

// outboxEntry is a narrow view of a claimed outbox row so that the processing
// loop can work with the generated ClaimPendingOutboxRow without leaking the
// sqlc type into helper signatures.
type outboxEntry struct {
	ID          uuid.UUID
	PayloadType string
	Payload     string
	RetryCount  *int32
}

func (p *OutboxProcessor) processEntry(ctx context.Context, id uuid.UUID, payloadType, payload string) error {
	switch payloadType {
	case "MESSAGE_SYNC":
		_, err := model.SyncMessageFromJSON(payload)
		if err != nil {
			return fmt.Errorf("parse message sync payload: %w", err)
		}
		slog.Info("Processing message sync outbox entry", "id", id)
		if p.eventPub != nil {
			// Remote changes are visible to every workspace client.
			p.eventPub.PublishRefresh(MessageListPanel)
		}
		return nil
	case "CONTACT_SYNC":
		slog.Info("Processing contact sync outbox entry", "id", id)
		if p.eventPub != nil {
			// Remote changes are visible to every workspace client.
			p.eventPub.PublishRefresh(ContactListPanel)
		}
		return nil
	default:
		slog.Warn("Unknown outbox payload type", "id", id, "type", payloadType)
		return fmt.Errorf("unknown payload type: %s", payloadType)
	}
}

// retryValue safely dereferences the nullable retry_count pointer for logging.
func retryValue(rc *int32) int32 {
	if rc == nil {
		return 0
	}
	return *rc
}
