package models

import (
	"time"

	"github.com/google/uuid"
)

type Vault struct {
	ID           uuid.UUID `json:"id"`
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	Salt         string    `json:"salt"`
	Verification string    `json:"verification"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type VaultItem struct {
	ID        uuid.UUID `json:"id"`
	VaultID   uuid.UUID `json:"vault_id"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
