package service

import "log/slog"

type EmailService interface {
	Send(to, subject, body string)
}

type NoOpEmailService struct{}

func (n *NoOpEmailService) Send(to, subject, body string) {}

type ConsoleEmailService struct{}

func (c *ConsoleEmailService) Send(to, subject, body string) {
	slog.Info(
		"Sending email",
		"to", to,
		"subject", subject,
		"body", body,
	)
}
