package services

import (
	"testing"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestEmailService_IsConfigured_True(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     "587",
		Username: "user@example.com",
		Password: "password",
		From:     "noreply@example.com",
	}
	svc := NewEmailService(cfg)

	assert.True(t, svc.IsConfigured())
}

func TestEmailService_IsConfigured_MissingHost(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "",
		Port:     "587",
		Username: "user@example.com",
		Password: "password",
		From:     "noreply@example.com",
	}
	svc := NewEmailService(cfg)

	assert.False(t, svc.IsConfigured())
}

func TestEmailService_IsConfigured_MissingUsername(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     "587",
		Username: "",
		Password: "password",
		From:     "noreply@example.com",
	}
	svc := NewEmailService(cfg)

	assert.False(t, svc.IsConfigured())
}

func TestEmailService_IsConfigured_MissingPassword(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     "587",
		Username: "user@example.com",
		Password: "",
		From:     "noreply@example.com",
	}
	svc := NewEmailService(cfg)

	assert.False(t, svc.IsConfigured())
}

func TestEmailService_IsConfigured_MissingFrom(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     "587",
		Username: "user@example.com",
		Password: "password",
		From:     "",
	}
	svc := NewEmailService(cfg)

	assert.False(t, svc.IsConfigured())
}

func TestEmailService_Send_NotConfigured(t *testing.T) {
	cfg := config.SMTPConfig{}
	svc := NewEmailService(cfg)

	err := svc.Send("to@example.com", "Subject", "Body")

	assert.NoError(t, err)
}

func TestEmailService_SendTeamInvite_NotConfigured(t *testing.T) {
	cfg := config.SMTPConfig{}
	svc := NewEmailService(cfg)

	err := svc.SendTeamInvite("to@example.com", "Test Team", "John Doe", "http://example.com/invite/123")

	assert.NoError(t, err)
}
