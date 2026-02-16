package models

import (
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	TeamID    *uuid.UUID `json:"team_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (w *Workspace) IsPersonal() bool {
	return w.UserID != nil
}

func (w *Workspace) IsTeam() bool {
	return w.TeamID != nil
}
