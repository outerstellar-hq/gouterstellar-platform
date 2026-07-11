package platform

import (
	"context"

	"github.com/rygel/gouterstellar-platform/internal/service"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

// MessageCounterAdapter wraps *service.MessageService as a platform.MessageCounter.
type MessageCounterAdapter struct {
	svc *service.MessageService
}

func NewMessageCounterAdapter(svc *service.MessageService) *MessageCounterAdapter {
	return &MessageCounterAdapter{svc: svc}
}

func (a *MessageCounterAdapter) CountMessages(ctx context.Context) (int64, error) {
	return a.svc.CountMessages(ctx)
}

// ContactCounterAdapter wraps *service.ContactService as a platform.ContactCounter.
type ContactCounterAdapter struct {
	svc *service.ContactService
}

func NewContactCounterAdapter(svc *service.ContactService) *ContactCounterAdapter {
	return &ContactCounterAdapter{svc: svc}
}

func (a *ContactCounterAdapter) CountContacts(ctx context.Context) (int64, error) {
	return a.svc.CountContacts(ctx)
}

// UserCounterAdapter wraps *service.SecurityService as a platform.UserCounter.
type UserCounterAdapter struct {
	svc *service.SecurityService
}

func NewUserCounterAdapter(svc *service.SecurityService) *UserCounterAdapter {
	return &UserCounterAdapter{svc: svc}
}

func (a *UserCounterAdapter) CountUsers(ctx context.Context) (int64, error) {
	return a.svc.CountUsers(ctx)
}

// BuildServiceBag creates a platform.ServiceBag from the internal services.
func BuildServiceBag(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	secSvc *service.SecurityService,
) extplatform.ServiceBag {
	return extplatform.ServiceBag{
		MessageCounter: NewMessageCounterAdapter(msgSvc),
		ContactCounter: NewContactCounterAdapter(contactSvc),
		UserCounter:    NewUserCounterAdapter(secSvc),
	}
}
