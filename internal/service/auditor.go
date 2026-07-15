package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/persistence"
)

// Auditor is the single sink for audit-log writes across the service layer.
// Services that previously held their own AuditRepository and inlined a
// LogAudit call now depend on this interface so the write path is uniform and
// the repository dependency stays out of services that only ever write.
//
// Read access to the audit log (list/page/count) stays on SecurityService,
// which legitimately needs the AuditRepository for its audit-read methods; only
// the write path is centralized here.
type Auditor interface {
	Record(ctx context.Context, action string, actorID *uuid.UUID, actorName *string, targetID *uuid.UUID, targetName *string, detail string)
}

// auditService is the canonical Auditor implementation: it wraps an
// AuditRepository and logs failures via slog without propagating them, matching
// the prior best-effort behavior of every service's private auditLog helper.
type auditService struct {
	repo persistence.AuditRepository
}

// NewAuditService wraps the given AuditRepository in an Auditor. Write failures
// are logged but never returned, so an audit hiccup cannot fail the originating
// request.
func NewAuditService(repo persistence.AuditRepository) Auditor {
	return &auditService{repo: repo}
}

func (a *auditService) Record(ctx context.Context, action string, actorID *uuid.UUID, actorName *string, targetID *uuid.UUID, targetName *string, detail string) {
	_, err := a.repo.LogAudit(ctx, actorID, actorName, targetID, targetName, action, detail)
	if err != nil {
		slog.Error("Failed to log audit entry", "action", action, "error", err)
	}
}
