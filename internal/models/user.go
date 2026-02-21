package models

import (
	"time"

	"github.com/google/uuid"
)

// Global user roles (platform-wide permissions)
const (
	GlobalRoleSuperAdmin = "super_admin"
	GlobalRoleUser       = "user"
)

type User struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	AvatarURL  *string   `json:"avatar_url,omitempty"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"-"`
	GlobalRole string    `json:"global_role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
