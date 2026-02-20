package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkspaceAPIKey struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	Name        string     `json:"name"`
	KeyHash     string     `json:"-"`
	KeyPrefix   string     `json:"key_prefix"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
