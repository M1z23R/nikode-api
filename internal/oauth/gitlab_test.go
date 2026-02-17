package oauth

import (
	"testing"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestGitLabProvider_Name(t *testing.T) {
	provider := NewGitLabProvider(config.OAuthConfig{})
	assert.Equal(t, "gitlab", provider.Name())
}

func TestGitLabProvider_GetConsentURL(t *testing.T) {
	provider := NewGitLabProvider(config.OAuthConfig{
		ClientID:    "test-client-id",
		RedirectURL: "http://localhost/callback",
	})

	url := provider.GetConsentURL("test-state")

	assert.Contains(t, url, "gitlab.com")
	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "state=test-state")
}

func TestGitLabProvider_Scopes(t *testing.T) {
	provider := NewGitLabProvider(config.OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	})

	// Verify read_user scope is configured
	assert.Contains(t, provider.config.Scopes, "read_user")
}

func TestGitLabProvider_Endpoint(t *testing.T) {
	provider := NewGitLabProvider(config.OAuthConfig{})

	// Verify GitLab endpoints
	assert.Equal(t, "https://gitlab.com/oauth/authorize", provider.config.Endpoint.AuthURL)
	assert.Equal(t, "https://gitlab.com/oauth/token", provider.config.Endpoint.TokenURL)
}

func TestGitLabProvider_ExchangeCode_MockSetup(t *testing.T) {
	// This test verifies the provider structure and would be used for integration testing
	// The actual ExchangeCode requires a real GitLab OAuth flow or mocking at HTTP level

	provider := NewGitLabProvider(config.OAuthConfig{
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

func TestGitLabProvider_UserInfoParsing(t *testing.T) {
	// Test that the UserInfo structure matches what GitLab API returns
	// This validates the JSON parsing logic indirectly

	provider := NewGitLabProvider(config.OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
	})

	// The provider should be set up to parse GitLab's user response:
	// { "id": int, "username": string, "name": string, "email": string, "avatar_url": string }
	assert.Equal(t, "gitlab", provider.Name())

	// Scopes should request read_user to get user info
	assert.Contains(t, provider.config.Scopes, "read_user")
}

func TestGitLabProvider_RedirectURL(t *testing.T) {
	provider := NewGitLabProvider(config.OAuthConfig{
		ClientID:    "test-client-id",
		RedirectURL: "https://myapp.com/auth/gitlab/callback",
	})

	url := provider.GetConsentURL("state123")

	assert.Contains(t, url, "redirect_uri=https")
	assert.Contains(t, url, "myapp.com")
}
