package service

type EventPublisher interface {
	PublishRefresh(targetID string)
}

type NoOpEventPublisher struct{}

func (n *NoOpEventPublisher) PublishRefresh(targetID string) {}
