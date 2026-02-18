package dto

import "github.com/google/uuid"

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

type UpdateWorkspaceRequest struct {
	Name string `json:"name"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
}

type WorkspaceResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	OwnerID uuid.UUID `json:"owner_id"`
	Role    string    `json:"role"`
}

type WorkspaceMemberResponse struct {
	ID     uuid.UUID    `json:"id"`
	UserID uuid.UUID    `json:"user_id"`
	Role   string       `json:"role"`
	User   UserResponse `json:"user"`
}

type WorkspaceInviteResponse struct {
	ID          uuid.UUID          `json:"id"`
	WorkspaceID uuid.UUID          `json:"workspace_id"`
	Status      string             `json:"status"`
	CreatedAt   string             `json:"created_at"`
	Workspace   *WorkspaceResponse `json:"workspace,omitempty"`
	Inviter     *UserResponse      `json:"inviter,omitempty"`
	Invitee     *UserResponse      `json:"invitee,omitempty"`
}
