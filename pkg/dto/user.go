package dto

import "github.com/google/uuid"

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	Provider  string    `json:"provider"`
}

type UpdateUserRequest struct {
	Name string `json:"name"`
}
