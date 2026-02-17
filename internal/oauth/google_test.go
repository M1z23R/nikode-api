package oauth

import (
	"testing"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2/google"
)

func TestGoogleProvider_Name(t *testing.T) {
	provider := NewGoogleProvider(config.OAuthConfig{})
	assert.Equal(t, "google", provider.Name())
}

func TestGoogleProvider_GetConsentURL(t *testing.T) {
	provider := NewGoogleProvider(config.OAuthConfig{
		ClientID:    "test-client-id",
		RedirectURL: "http://localhost/callback",
	})

	url := provider.GetConsentURL("test-state")

	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "state=test-state")
}

func TestGoogleProvider_Scopes(t *testing.T) {
	provider := NewGoogleProvider(config.OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	})

	// Verify required scopes are configured
	assert.Contains(t, provider.config.Scopes, "https://www.googleapis.com/auth/userinfo.email")
	assert.Contains(t, provider.config.Scopes, "https://www.googleapis.com/auth/userinfo.profile")
}

func TestGoogleProvider_Endpoint(t *testing.T) {
	provider := NewGoogleProvider(config.OAuthConfig{})

	// Verify Google endpoints
	assert.Equal(t, google.Endpoint.AuthURL, provider.config.Endpoint.AuthURL)
	assert.Equal(t, google.Endpoint.TokenURL, provider.config.Endpoint.TokenURL)
}
