package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Collection struct {
	ID          uuid.UUID       `json:"id"`
	WorkspaceID uuid.UUID       `json:"workspace_id"`
	Name        string          `json:"name"`
	Data        json.RawMessage `json:"data"`
	Version     int             `json:"version"`
	UpdatedBy   *uuid.UUID      `json:"updated_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
