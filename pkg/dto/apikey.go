package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type APIKeyResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	ExpiresAt *string    `json:"expires_at,omitempty"`
	CreatedAt string     `json:"created_at"`
	LastUsedAt *string   `json:"last_used_at,omitempty"`
}

type APIKeyCreatedResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"`
	KeyPrefix string     `json:"key_prefix"`
	ExpiresAt *string    `json:"expires_at,omitempty"`
	CreatedAt string     `json:"created_at"`
}
