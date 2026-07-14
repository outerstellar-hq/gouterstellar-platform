package service

import (
	"fmt"
	"log/slog"
	"net/smtp"
)

type SmtpConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	StartTLS bool
}

type SmtpEmailService struct {
	config SmtpConfig
}

func NewSmtpEmailService(config SmtpConfig) *SmtpEmailService {
	return &SmtpEmailService{config: config}
}

// Send delivers the message via SMTP. smtp.SendMail already upgrades to TLS
// opportunistically when the server advertises STARTTLS, so an explicit
// StartTLS branch is unnecessary here. Forcing STARTTLS (e.g. against a server
// that does not advertise it) or configuring a custom TLS dialer is not
// implemented for this PoC.
func (s *SmtpEmailService) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.config.From, to, subject, body)

	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	if err := smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	slog.Info("Email sent", "to", to, "subject", subject)
	return nil
}
