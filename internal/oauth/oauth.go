package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
)

type UserInfo struct {
	Email     string
	Name      string
	AvatarURL string
	ID        string
	Provider  string
}

type Provider interface {
	GetConsentURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*UserInfo, error)
	Name() string
}

func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
