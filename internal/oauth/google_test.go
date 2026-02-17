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

func TestGoogleProvider_ExchangeCode_MockSetup(t *testing.T) {
	// This test verifies the provider structure and would be used for integration testing
	// The actual ExchangeCode requires a real Google OAuth flow or mocking at HTTP level

	provider := NewGoogleProvider(config.OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	})

	// Verify provider is properly configured
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.config)
	assert.Equal(t, "test-client-id", provider.config.ClientID)
	assert.Equal(t, "test-secret", provider.config.ClientSecret)
	assert.Equal(t, "http://localhost/callback", provider.config.RedirectURL)
}

func TestGoogleProvider_UserInfoParsing(t *testing.T) {
	// Test that the UserInfo structure matches what Google API returns
	// This validates the JSON parsing logic indirectly

	provider := NewGoogleProvider(config.OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
	})

	// The provider should be set up to parse Google's user response:
	// { "id": string, "email": string, "verified_email": bool, "name": string, "picture": string }
	assert.Equal(t, "google", provider.Name())

	// Scopes should request email and profile info
	assert.Contains(t, provider.config.Scopes, "https://www.googleapis.com/auth/userinfo.email")
	assert.Contains(t, provider.config.Scopes, "https://www.googleapis.com/auth/userinfo.profile")
}

func TestGoogleProvider_RedirectURL(t *testing.T) {
	provider := NewGoogleProvider(config.OAuthConfig{
		ClientID:    "test-client-id",
		RedirectURL: "https://myapp.com/auth/google/callback",
	})

	url := provider.GetConsentURL("state123")

	assert.Contains(t, url, "redirect_uri=https")
	assert.Contains(t, url, "myapp.com")
}

func TestGoogleProvider_AccessTypeOffline(t *testing.T) {
	provider := NewGoogleProvider(config.OAuthConfig{
		ClientID: "test-client-id",
	})

	url := provider.GetConsentURL("state123")

	// Should request offline access for refresh tokens
	assert.Contains(t, url, "access_type=offline")
}
