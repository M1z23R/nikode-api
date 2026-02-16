package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

type CreateCollectionRequest struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

type UpdateCollectionRequest struct {
	Name    *string         `json:"name,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Version int             `json:"version"`
}

type CollectionResponse struct {
	ID          uuid.UUID       `json:"id"`
	WorkspaceID uuid.UUID       `json:"workspace_id"`
	Name        string          `json:"name"`
	Data        json.RawMessage `json:"data"`
	Version     int             `json:"version"`
	UpdatedBy   *uuid.UUID      `json:"updated_by,omitempty"`
}
