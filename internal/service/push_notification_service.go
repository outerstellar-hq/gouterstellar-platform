package service

import "log/slog"

type PushNotification struct {
	Title string
	Body  string
	Data  map[string]string
}

type PushNotificationService interface {
	Send(platform, deviceToken string, notification PushNotification)
}

type ConsolePushNotificationService struct{}

func (c *ConsolePushNotificationService) Send(platform, deviceToken string, notification PushNotification) {
	slog.Info(
		"Push notification",
		"platform", platform,
		"title", notification.Title,
	)
}
