package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

type TemplateSearchResult struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
}

type TemplateDetail struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Data        json.RawMessage `json:"data"`
}

type CreateTemplateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Data        json.RawMessage `json:"data"`
}
