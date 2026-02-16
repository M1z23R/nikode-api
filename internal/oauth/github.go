package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dimitrije/nikode-api/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GitHubProvider struct {
	config *oauth2.Config
}

func NewGitHubProvider(cfg config.OAuthConfig) *GitHubProvider {
	return &GitHubProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     github.Endpoint,
		},
	}
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) GetConsentURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*UserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := p.config.Client(ctx, token)

	userResp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", userResp.StatusCode)
	}

	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	email := ghUser.Email
	if email == "" {
		email, err = p.getPrimaryEmail(ctx, client)
		if err != nil {
			return nil, err
		}
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	return &UserInfo{
		Email:     email,
		Name:      name,
		AvatarURL: ghUser.AvatarURL,
		ID:        fmt.Sprintf("%d", ghUser.ID),
		Provider:  "github",
	}, nil
}

func (p *GitHubProvider) getPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", fmt.Errorf("failed to get user emails: %w", err)
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("failed to decode emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", fmt.Errorf("no email found")
}
