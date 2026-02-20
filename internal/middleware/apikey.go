package middleware

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

const (
	APIKeyWorkspaceIDKey = "api_key_workspace_id"
)

// APIKeyServiceInterface defines the methods needed by the API key middleware
type APIKeyServiceInterface interface {
	ValidateAndGetWorkspace(ctx context.Context, key string) (uuid.UUID, error)
}

// APIKeyAuth creates middleware that authenticates requests using API keys
func APIKeyAuth(apiKeyService APIKeyServiceInterface) drift.HandlerFunc {
	return func(c *drift.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Unauthorized("missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Unauthorized("invalid authorization header format")
			return
		}

		token := parts[1]

		// Check if it's an API key (starts with nik_)
		if !strings.HasPrefix(token, "nik_") {
			c.Unauthorized("invalid api key format")
			return
		}

		workspaceID, err := apiKeyService.ValidateAndGetWorkspace(context.Background(), token)
		if err != nil {
			c.Unauthorized("invalid or expired api key")
			return
		}

		c.Set(APIKeyWorkspaceIDKey, workspaceID)
		c.Next()
	}
}

// GetAPIKeyWorkspaceID retrieves the workspace ID from context (set by API key auth)
func GetAPIKeyWorkspaceID(c *drift.Context) uuid.UUID {
	if id, ok := c.Get(APIKeyWorkspaceIDKey); ok {
		if uid, ok := id.(uuid.UUID); ok {
			return uid
		}
	}
	return uuid.Nil
}
