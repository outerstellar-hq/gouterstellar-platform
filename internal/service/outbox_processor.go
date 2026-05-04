package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
)

type OutboxProcessor struct {
	outboxRepo persistence.OutboxRepository
	txMgr      *persistence.TransactionManager
}

func NewOutboxProcessor(
	outboxRepo persistence.OutboxRepository,
	txMgr *persistence.TransactionManager,
) *OutboxProcessor {
	return &OutboxProcessor{
		outboxRepo: outboxRepo,
		txMgr:      txMgr,
	}
}

func (p *OutboxProcessor) ProcessPending(ctx context.Context) error {
	entries, err := p.outboxRepo.ListPending(ctx, 10)
	if err != nil {
		return fmt.Errorf("list pending outbox: %w", err)
	}

	for _, entry := range entries {
		if err := p.processEntry(ctx, entry.ID, entry.PayloadType, entry.Payload); err != nil {
			errMsg := err.Error()
			_, markErr := p.outboxRepo.MarkFailed(ctx, entry.ID, &errMsg)
			if markErr != nil {
				slog.Error("Failed to mark outbox entry as failed", "id", entry.ID, "error", markErr)
			}
			slog.Error("Failed to process outbox entry", "id", entry.ID, "error", err)
			continue
		}

		_, err := p.outboxRepo.MarkProcessed(ctx, entry.ID)
		if err != nil {
			slog.Error("Failed to mark outbox entry as processed", "id", entry.ID, "error", err)
		}
	}

	return nil
}

func (p *OutboxProcessor) processEntry(ctx context.Context, id uuid.UUID, payloadType, payload string) error {
	switch payloadType {
	case "MESSAGE_SYNC":
		_, err := model.SyncMessageFromJSON(payload)
		if err != nil {
			return fmt.Errorf("parse message sync payload: %w", err)
		}
		slog.Info("Processing message sync outbox entry", "id", id)
		return nil
	case "CONTACT_SYNC":
		slog.Info("Processing contact sync outbox entry", "id", id)
		return nil
	default:
		slog.Warn("Unknown outbox payload type", "id", id, "type", payloadType)
		return fmt.Errorf("unknown payload type: %s", payloadType)
	}
}
