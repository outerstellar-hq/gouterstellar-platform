package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
)

// WritePipeline centralizes the post-mutation side-effects that every
// MessageService write used to inline: cache invalidation, refresh-event
// publish, in-app notification, and notification email. Collapsing these into
// one type keeps the mutation methods focused on the write itself and
// guarantees side-effects fire in a consistent order.
//
// The pipeline is nil-safe: every method is a no-op when called on a nil
// receiver, so a service that opts out of a side-effect (e.g. no email) can
// pass nil for the relevant dependency.
type WritePipeline struct {
	cache    *persistence.MessageCache
	eventPub EventPublisher
	notifier *NotificationService
	emailSvc EmailService
}

// NewWritePipeline assembles a WritePipeline from its dependencies. Any
// dependency may be nil; the pipeline skips nil-backed side-effects at call
// time.
func NewWritePipeline(cache *persistence.MessageCache, eventPub EventPublisher, notifier *NotificationService, emailSvc EmailService) *WritePipeline {
	return &WritePipeline{
		cache:    cache,
		eventPub: eventPub,
		notifier: notifier,
		emailSvc: emailSvc,
	}
}

// AfterMessageChange runs the common post-mutation side-effects for a single
// message: drop the per-message cache entry and the list cache prefix, then
// broadcast a refresh. This is the path shared by update, delete, restore, and
// conflict resolution.
func (p *WritePipeline) AfterMessageChange(ctx context.Context, syncID string) {
	if p == nil {
		return
	}
	if p.cache != nil {
		p.cache.Invalidate("message:" + syncID)
		p.cache.InvalidateByPrefix("messages:")
	}
	if p.eventPub != nil {
		p.eventPub.PublishRefresh(MessageListPanel)
	}
}

// AfterMessageCreated runs the same cache/event side-effects as
// AfterMessageChange (for a freshly created message there is no per-message
// cache entry to drop yet, but invalidating the prefix is sufficient and the
// per-key invalidate is harmless) and additionally records a best-effort
// in-app notification and notification email for the acting user. Notification
// and email failures are logged but never propagated.
func (p *WritePipeline) AfterMessageCreated(ctx context.Context, author, content string) {
	if p == nil {
		return
	}
	if p.cache != nil {
		p.cache.InvalidateByPrefix("messages:")
	}
	if p.eventPub != nil {
		p.eventPub.PublishRefresh(MessageListPanel)
	}
	p.notifyActor(ctx, "New Message", truncateContent(content), "message")
	p.notifyByEmail(ctx, "New message created",
		fmt.Sprintf("A new message was created:\n\nAuthor: %s\nContent: %s", author, content))
}

// notifyActor records a best-effort notification for the user acting on the
// current request. Nil-safe: no notifier wired or no authenticated user means
// nothing happens. Failures are logged but never propagated.
func (p *WritePipeline) notifyActor(ctx context.Context, title, body, nType string) {
	if p.notifier == nil {
		return
	}
	user := UserFromContext(ctx)
	if user == nil {
		return
	}
	if err := p.notifier.Create(ctx, user.ID, title, body, nType); err != nil {
		slog.Warn("Failed to create notification", "title", title, "error", err)
	}
}

// notifyByEmail sends a best-effort notification email to the acting user when
// they have email notifications enabled. Nil-safe; failures are logged only.
func (p *WritePipeline) notifyByEmail(ctx context.Context, subject, body string) {
	if p.emailSvc == nil {
		return
	}
	user := UserFromContext(ctx)
	if user == nil || !user.EmailNotificationsEnabled || user.Email == "" {
		return
	}
	if err := p.emailSvc.Send(user.Email, subject, body); err != nil {
		slog.Error("Failed to send notification email", "subject", subject, "error", err)
	}
}
