package dto

import "github.com/google/uuid"

type CreateTeamRequest struct {
	Name string `json:"name"`
}

type UpdateTeamRequest struct {
	Name string `json:"name"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
}

type TeamResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	OwnerID uuid.UUID `json:"owner_id"`
	Role    string    `json:"role"`
}

type TeamMemberResponse struct {
	ID     uuid.UUID    `json:"id"`
	UserID uuid.UUID    `json:"user_id"`
	Role   string       `json:"role"`
	User   UserResponse `json:"user"`
}

type TeamInviteResponse struct {
	ID        uuid.UUID     `json:"id"`
	TeamID    uuid.UUID     `json:"team_id"`
	Status    string        `json:"status"`
	CreatedAt string        `json:"created_at"`
	Team      *TeamResponse `json:"team,omitempty"`
	Inviter   *UserResponse `json:"inviter,omitempty"`
	Invitee   *UserResponse `json:"invitee,omitempty"`
}
