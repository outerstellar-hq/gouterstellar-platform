package service

// EventPublisher broadcasts refresh signals to connected clients. PublishRefresh
// accepts an optional userID: when non-empty, the refresh is delivered only to
// clients belonging to that user; when empty, it is broadcast to every client.
type EventPublisher interface {
	PublishRefresh(userID, targetID string)
}

type NoOpEventPublisher struct{}

func (n *NoOpEventPublisher) PublishRefresh(userID, targetID string) {}
