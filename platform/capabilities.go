package platform

import "context"

// MessageCounter is the capability for reading message counts without
// depending on internal service types.
type MessageCounter interface {
	CountMessages(ctx context.Context) (int64, error)
}

// ServiceBag carries platform-level capabilities that extensions can request.
// The wire root populates this by adapting internal services.
type ServiceBag struct {
	MessageCounter MessageCounter
}
