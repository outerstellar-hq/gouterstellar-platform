package service

const (
	MessageListPanel = "message-list-panel"
	ContactListPanel = "contact-list-panel"
)

// EventPublisher broadcasts refresh signals to connected clients.
type EventPublisher interface {
	PublishRefresh(targetID string)
}

type NoOpEventPublisher struct{}

func (n *NoOpEventPublisher) PublishRefresh(targetID string) {}
