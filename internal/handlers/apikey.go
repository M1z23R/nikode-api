package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type APIKeyHandler struct {
	apiKeyService    APIKeyServiceInterface
	workspaceService WorkspaceServiceInterface
}

func NewAPIKeyHandler(apiKeyService APIKeyServiceInterface, workspaceService WorkspaceServiceInterface) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService:    apiKeyService,
		workspaceService: workspaceService,
	}
}

func (h *APIKeyHandler) Create(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	// Only workspace owners can create API keys
	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can create api keys")
		return
	}

	var req dto.CreateAPIKeyRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.BadRequest("name is required")
		return
	}

	apiKey, plainKey, err := h.apiKeyService.Create(context.Background(), workspaceID, req.Name, userID, req.ExpiresAt)
	if err != nil {
		c.InternalServerError("failed to create api key")
		return
	}

	response := dto.APIKeyCreatedResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Key:       plainKey,
		KeyPrefix: apiKey.KeyPrefix,
		CreatedAt: apiKey.CreatedAt.Format(time.RFC3339),
	}
	if apiKey.ExpiresAt != nil {
		formatted := apiKey.ExpiresAt.Format(time.RFC3339)
		response.ExpiresAt = &formatted
	}

	_ = c.JSON(201, response)
}

func (h *APIKeyHandler) List(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	// Only workspace owners can list API keys
	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can list api keys")
		return
	}

	keys, err := h.apiKeyService.List(context.Background(), workspaceID)
	if err != nil {
		c.InternalServerError("failed to list api keys")
		return
	}

	var response []dto.APIKeyResponse
	for _, k := range keys {
		item := dto.APIKeyResponse{
			ID:        k.ID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			CreatedAt: k.CreatedAt.Format(time.RFC3339),
		}
		if k.ExpiresAt != nil {
			formatted := k.ExpiresAt.Format(time.RFC3339)
			item.ExpiresAt = &formatted
		}
		if k.LastUsedAt != nil {
			formatted := k.LastUsedAt.Format(time.RFC3339)
			item.LastUsedAt = &formatted
		}
		response = append(response, item)
	}

	if response == nil {
		response = []dto.APIKeyResponse{}
	}

	_ = c.JSON(200, response)
}

func (h *APIKeyHandler) Revoke(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	keyID, err := uuid.Parse(c.Param("keyId"))
	if err != nil {
		c.BadRequest("invalid key id")
		return
	}

	// Only workspace owners can revoke API keys
	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can revoke api keys")
		return
	}

	err = h.apiKeyService.Revoke(context.Background(), keyID, workspaceID)
	if err != nil {
		c.NotFound("api key not found")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "api key revoked"})
}
