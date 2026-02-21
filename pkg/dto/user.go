package dto

import "github.com/google/uuid"

type UserResponse struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	AvatarURL  *string   `json:"avatar_url,omitempty"`
	Provider   string    `json:"provider"`
	GlobalRole string    `json:"global_role"`
}

type UpdateUserRequest struct {
	Name string `json:"name"`
}
