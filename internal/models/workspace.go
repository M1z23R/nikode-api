package models

import (
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	OwnerID   uuid.UUID `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WorkspaceMember struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	UserID      uuid.UUID `json:"user_id"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	User        *User     `json:"user,omitempty"`
}

type WorkspaceInvite struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	InviterID   uuid.UUID  `json:"inviter_id"`
	InviteeID   uuid.UUID  `json:"invitee_id"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Workspace   *Workspace `json:"workspace,omitempty"`
	Inviter     *User      `json:"inviter,omitempty"`
	Invitee     *User      `json:"invitee,omitempty"`
}

const (
	RoleOwner  = "owner"
	RoleMember = "member"

	InviteStatusPending  = "pending"
	InviteStatusAccepted = "accepted"
	InviteStatusDeclined = "declined"
)
