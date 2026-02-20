package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type VaultHandler struct {
	vaultService     VaultServiceInterface
	workspaceService WorkspaceServiceInterface
}

func NewVaultHandler(vaultService VaultServiceInterface, workspaceService WorkspaceServiceInterface) *VaultHandler {
	return &VaultHandler{
		vaultService:     vaultService,
		workspaceService: workspaceService,
	}
}

func (h *VaultHandler) CreateVault(c *drift.Context) {
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

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can create a vault")
		return
	}

	var req dto.CreateVaultRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if strings.TrimSpace(req.Salt) == "" || strings.TrimSpace(req.Verification) == "" {
		c.BadRequest("salt and verification are required")
		return
	}

	vault, err := h.vaultService.Create(context.Background(), workspaceID, req.Salt, req.Verification)
	if err != nil {
		if err == services.ErrVaultAlreadyExists {
			c.BadRequest("vault already exists for this workspace")
			return
		}
		c.InternalServerError("failed to create vault")
		return
	}

	_ = c.JSON(201, dto.VaultResponse{
		ID:           vault.ID,
		Salt:         vault.Salt,
		Verification: vault.Verification,
		CreatedAt:    vault.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    vault.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *VaultHandler) GetVault(c *drift.Context) {
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

	canAccess, err := h.workspaceService.CanAccess(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check access")
		return
	}
	if !canAccess {
		c.Forbidden("you do not have access to this workspace")
		return
	}

	vault, err := h.vaultService.GetByWorkspaceID(context.Background(), workspaceID)
	if err != nil {
		if err == services.ErrVaultNotFound {
			c.NotFound("vault not found")
			return
		}
		c.InternalServerError("failed to get vault")
		return
	}

	_ = c.JSON(200, dto.VaultResponse{
		ID:           vault.ID,
		Salt:         vault.Salt,
		Verification: vault.Verification,
		CreatedAt:    vault.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    vault.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *VaultHandler) DeleteVault(c *drift.Context) {
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

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can delete the vault")
		return
	}

	err = h.vaultService.Delete(context.Background(), workspaceID)
	if err != nil {
		if err == services.ErrVaultNotFound {
			c.NotFound("vault not found")
			return
		}
		c.InternalServerError("failed to delete vault")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "vault deleted"})
}

func (h *VaultHandler) ListItems(c *drift.Context) {
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

	canAccess, err := h.workspaceService.CanAccess(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check access")
		return
	}
	if !canAccess {
		c.Forbidden("you do not have access to this workspace")
		return
	}

	vault, err := h.vaultService.GetByWorkspaceID(context.Background(), workspaceID)
	if err != nil {
		if err == services.ErrVaultNotFound {
			c.NotFound("vault not found")
			return
		}
		c.InternalServerError("failed to get vault")
		return
	}

	items, err := h.vaultService.ListItems(context.Background(), vault.ID)
	if err != nil {
		c.InternalServerError("failed to list vault items")
		return
	}

	var response []dto.VaultItemResponse
	for _, item := range items {
		response = append(response, dto.VaultItemResponse{
			ID:        item.ID,
			Data:      item.Data,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
			UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
		})
	}

	if response == nil {
		response = []dto.VaultItemResponse{}
	}

	_ = c.JSON(200, response)
}

func (h *VaultHandler) CreateItem(c *drift.Context) {
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

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can create vault items")
		return
	}

	vault, err := h.vaultService.GetByWorkspaceID(context.Background(), workspaceID)
	if err != nil {
		if err == services.ErrVaultNotFound {
			c.NotFound("vault not found")
			return
		}
		c.InternalServerError("failed to get vault")
		return
	}

	var req dto.CreateVaultItemRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if strings.TrimSpace(req.Data) == "" {
		c.BadRequest("data is required")
		return
	}

	item, err := h.vaultService.CreateItem(context.Background(), vault.ID, req.Data)
	if err != nil {
		c.InternalServerError("failed to create vault item")
		return
	}

	_ = c.JSON(201, dto.VaultItemResponse{
		ID:        item.ID,
		Data:      item.Data,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *VaultHandler) UpdateItem(c *drift.Context) {
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

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.BadRequest("invalid item id")
		return
	}

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can update vault items")
		return
	}

	vault, err := h.vaultService.GetByWorkspaceID(context.Background(), workspaceID)
	if err != nil {
		if err == services.ErrVaultNotFound {
			c.NotFound("vault not found")
			return
		}
		c.InternalServerError("failed to get vault")
		return
	}

	var req dto.UpdateVaultItemRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if strings.TrimSpace(req.Data) == "" {
		c.BadRequest("data is required")
		return
	}

	item, err := h.vaultService.UpdateItem(context.Background(), itemID, vault.ID, req.Data)
	if err != nil {
		if err == services.ErrVaultItemNotFound {
			c.NotFound("vault item not found")
			return
		}
		c.InternalServerError("failed to update vault item")
		return
	}

	_ = c.JSON(200, dto.VaultItemResponse{
		ID:        item.ID,
		Data:      item.Data,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *VaultHandler) DeleteItem(c *drift.Context) {
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

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.BadRequest("invalid item id")
		return
	}

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil {
		c.InternalServerError("failed to check ownership")
		return
	}
	if !isOwner {
		c.Forbidden("only workspace owners can delete vault items")
		return
	}

	vault, err := h.vaultService.GetByWorkspaceID(context.Background(), workspaceID)
	if err != nil {
		if err == services.ErrVaultNotFound {
			c.NotFound("vault not found")
			return
		}
		c.InternalServerError("failed to get vault")
		return
	}

	err = h.vaultService.DeleteItem(context.Background(), itemID, vault.ID)
	if err != nil {
		if err == services.ErrVaultItemNotFound {
			c.NotFound("vault item not found")
			return
		}
		c.InternalServerError("failed to delete vault item")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "vault item deleted"})
}
