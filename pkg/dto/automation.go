package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

type UpsertCollectionRequest struct {
	Name         string          `json:"name"`
	CollectionID string          `json:"collection_id,omitempty"`
	Resolution   string          `json:"resolution,omitempty"` // "force", "clone", "fail" (default: "force")
	Spec         json.RawMessage `json:"spec"`
}

type UpsertCollectionResponse struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	Created     bool      `json:"created"`
}
