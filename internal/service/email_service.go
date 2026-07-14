package service

import "log/slog"

type EmailService interface {
	Send(to, subject, body string) error
}

type NoOpEmailService struct{}

func (n *NoOpEmailService) Send(to, subject, body string) error { return nil }

type ConsoleEmailService struct{}

func (c *ConsoleEmailService) Send(to, subject, body string) error {
	slog.Info(
		"Sending email",
		"to", to,
		"subject", subject,
		"body", body,
	)
	return nil
}
