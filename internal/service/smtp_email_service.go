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

func (s *SmtpEmailService) Send(to, subject, body string) {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.config.From, to, subject, body)

	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	var err error
	if s.config.StartTLS {
		err = smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(msg))
	} else {
		err = smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(msg))
	}

	if err != nil {
		slog.Error("Failed to send email", "to", to, "subject", subject, "error", err)
		return
	}
	slog.Info("Email sent", "to", to, "subject", subject)
}
