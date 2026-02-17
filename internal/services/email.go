package services

import (
	"fmt"
	"net/smtp"

	"github.com/dimitrije/nikode-api/internal/config"
)

type EmailService struct {
	cfg config.SMTPConfig
}

func NewEmailService(cfg config.SMTPConfig) *EmailService {
	return &EmailService{cfg: cfg}
}

func (s *EmailService) IsConfigured() bool {
	return s.cfg.Host != "" && s.cfg.Username != "" && s.cfg.Password != "" && s.cfg.From != ""
}

func (s *EmailService) Send(to, subject, body string) error {
	if !s.IsConfigured() {
		return nil
	}

	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		s.cfg.From, to, subject, body)

	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}

func (s *EmailService) SendTeamInvite(to, teamName, inviterName, inviteURL string) error {
	subject := fmt.Sprintf("You've been invited to join %s", teamName)
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>Team Invitation</h2>
			<p>Hi,</p>
			<p><strong>%s</strong> has invited you to join the team <strong>%s</strong>.</p>
			<p><a href="%s">Click here to view and respond to this invitation</a></p>
		</body>
		</html>
	`, inviterName, teamName, inviteURL)

	return s.Send(to, subject, body)
}
