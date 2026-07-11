package platform

import "context"

// MessageCounter is the capability for reading message counts without
// depending on internal service types.
type MessageCounter interface {
	CountMessages(ctx context.Context) (int64, error)
}

// ContactCounter is the capability for reading contact counts.
type ContactCounter interface {
	CountContacts(ctx context.Context) (int64, error)
}

// UserCounter is the capability for reading user counts.
type UserCounter interface {
	CountUsers(ctx context.Context) (int64, error)
}

// ServiceBag carries platform-level capabilities that extensions can request.
// The wire root populates this by adapting internal services.
type ServiceBag struct {
	MessageCounter MessageCounter
	ContactCounter ContactCounter
	UserCounter    UserCounter
}
