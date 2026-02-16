package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Env         string
	DatabaseURL string

	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration

	FrontendCallbackURL string

	GitHub OAuthConfig
	GitLab OAuthConfig
	Google OAuthConfig
}

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	accessExpiry, err := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m"))
	if err != nil {
		accessExpiry = 15 * time.Minute
	}

	refreshExpiry, err := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h"))
	if err != nil {
		refreshExpiry = 168 * time.Hour
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		Env:         getEnv("ENV", "development"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessExpiry:  accessExpiry,
		JWTRefreshExpiry: refreshExpiry,

		FrontendCallbackURL: getEnv("FRONTEND_CALLBACK_URL", "nikode://auth/callback"),

		GitHub: OAuthConfig{
			ClientID:     getEnv("GITHUB_CLIENT_ID", ""),
			ClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GITHUB_REDIRECT_URL", ""),
		},
		GitLab: OAuthConfig{
			ClientID:     getEnv("GITLAB_CLIENT_ID", ""),
			ClientSecret: getEnv("GITLAB_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GITLAB_REDIRECT_URL", ""),
		},
		Google: OAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),
		},
	}, nil
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
