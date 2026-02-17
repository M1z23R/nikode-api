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
