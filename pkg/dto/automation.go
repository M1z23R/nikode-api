package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

type UpsertCollectionRequest struct {
	Name  string          `json:"name"`
	Spec  json.RawMessage `json:"spec"`
	Force bool            `json:"force,omitempty"`
}

type UpsertCollectionResponse struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	Created     bool      `json:"created"`
}
