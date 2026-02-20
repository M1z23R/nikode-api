package dto

import "github.com/google/uuid"

type CreateVaultRequest struct {
	Salt         string `json:"salt"`
	Verification string `json:"verification"`
}

type VaultResponse struct {
	ID           uuid.UUID `json:"id"`
	Salt         string    `json:"salt"`
	Verification string    `json:"verification"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

type CreateVaultItemRequest struct {
	Data string `json:"data"`
}

type UpdateVaultItemRequest struct {
	Data string `json:"data"`
}

type VaultItemResponse struct {
	ID        uuid.UUID `json:"id"`
	Data      string    `json:"data"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}
