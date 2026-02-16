package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dimitrije/nikode-api/internal/config"
	"golang.org/x/oauth2"
)

var gitlabEndpoint = oauth2.Endpoint{
	AuthURL:  "https://gitlab.com/oauth/authorize",
	TokenURL: "https://gitlab.com/oauth/token",
}

type GitLabProvider struct {
	config *oauth2.Config
}

func NewGitLabProvider(cfg config.OAuthConfig) *GitLabProvider {
	return &GitLabProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"read_user"},
			Endpoint:     gitlabEndpoint,
		},
	}
}

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

func (p *GitLabProvider) GetConsentURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *GitLabProvider) ExchangeCode(ctx context.Context, code string) (*UserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := p.config.Client(ctx, token)

	resp, err := client.Get("https://gitlab.com/api/v4/user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab api returned status %d", resp.StatusCode)
	}

	var glUser struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	name := glUser.Name
	if name == "" {
		name = glUser.Username
	}

	return &UserInfo{
		Email:     glUser.Email,
		Name:      name,
		AvatarURL: glUser.AvatarURL,
		ID:        fmt.Sprintf("%d", glUser.ID),
		Provider:  "gitlab",
	}, nil
}
