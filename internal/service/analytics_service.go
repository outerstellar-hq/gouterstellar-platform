package service

import "context"

type AnalyticsService interface {
	Track(ctx context.Context, event string, properties map[string]interface{})
}

type NoOpAnalyticsService struct{}

func (n *NoOpAnalyticsService) Track(ctx context.Context, event string, properties map[string]interface{}) {
}
