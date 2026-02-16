package dto

import "github.com/google/uuid"

type CreateWorkspaceRequest struct {
	Name   string     `json:"name"`
	TeamID *uuid.UUID `json:"team_id,omitempty"`
}

type UpdateWorkspaceRequest struct {
	Name string `json:"name"`
}

type WorkspaceResponse struct {
	ID     uuid.UUID  `json:"id"`
	Name   string     `json:"name"`
	UserID *uuid.UUID `json:"user_id,omitempty"`
	TeamID *uuid.UUID `json:"team_id,omitempty"`
	Type   string     `json:"type"`
}
