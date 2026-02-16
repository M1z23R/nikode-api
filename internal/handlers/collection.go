package handlers

import (
	"context"
	"errors"

	"github.com/m1z23r/drift/pkg/drift"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/internal/sse"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
)

type CollectionHandler struct {
	collectionService *services.CollectionService
	workspaceService  *services.WorkspaceService
	hub               *sse.Hub
}

func NewCollectionHandler(
	collectionService *services.CollectionService,
	workspaceService *services.WorkspaceService,
	hub *sse.Hub,
) *CollectionHandler {
	return &CollectionHandler{
		collectionService: collectionService,
		workspaceService:  workspaceService,
		hub:               hub,
	}
}

func (h *CollectionHandler) Create(c *drift.Context) {
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

	ctx := context.Background()

	canAccess, err := h.workspaceService.CanAccess(ctx, workspaceID, userID)
	if err != nil || !canAccess {
		c.NotFound("workspace not found")
		return
	}

	var req dto.CreateCollectionRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	collection, err := h.collectionService.Create(ctx, workspaceID, req.Name, req.Data, userID)
	if err != nil {
		c.InternalServerError("failed to create collection")
		return
	}

	c.JSON(201, dto.CollectionResponse{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
		Data:        collection.Data,
		Version:     collection.Version,
		UpdatedBy:   collection.UpdatedBy,
	})
}

func (h *CollectionHandler) List(c *drift.Context) {
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

	ctx := context.Background()

	canAccess, err := h.workspaceService.CanAccess(ctx, workspaceID, userID)
	if err != nil || !canAccess {
		c.NotFound("workspace not found")
		return
	}

	collections, err := h.collectionService.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		c.InternalServerError("failed to get collections")
		return
	}

	response := make([]dto.CollectionResponse, len(collections))
	for i, col := range collections {
		response[i] = dto.CollectionResponse{
			ID:          col.ID,
			WorkspaceID: col.WorkspaceID,
			Name:        col.Name,
			Data:        col.Data,
			Version:     col.Version,
			UpdatedBy:   col.UpdatedBy,
		}
	}

	c.JSON(200, response)
}

func (h *CollectionHandler) Get(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	collectionID, err := uuid.Parse(c.Param("collectionId"))
	if err != nil {
		c.BadRequest("invalid collection id")
		return
	}

	ctx := context.Background()

	collection, err := h.collectionService.GetByID(ctx, collectionID)
	if err != nil {
		c.NotFound("collection not found")
		return
	}

	canAccess, err := h.workspaceService.CanAccess(ctx, collection.WorkspaceID, userID)
	if err != nil || !canAccess {
		c.NotFound("collection not found")
		return
	}

	c.JSON(200, dto.CollectionResponse{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
		Data:        collection.Data,
		Version:     collection.Version,
		UpdatedBy:   collection.UpdatedBy,
	})
}

func (h *CollectionHandler) Update(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	collectionID, err := uuid.Parse(c.Param("collectionId"))
	if err != nil {
		c.BadRequest("invalid collection id")
		return
	}

	ctx := context.Background()

	existing, err := h.collectionService.GetByID(ctx, collectionID)
	if err != nil {
		c.NotFound("collection not found")
		return
	}

	canAccess, err := h.workspaceService.CanAccess(ctx, existing.WorkspaceID, userID)
	if err != nil || !canAccess {
		c.NotFound("collection not found")
		return
	}

	var req dto.UpdateCollectionRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Version == 0 {
		c.BadRequest("version is required for optimistic locking")
		return
	}

	collection, err := h.collectionService.Update(ctx, collectionID, req.Name, req.Data, req.Version, userID)
	if err != nil {
		if errors.Is(err, services.ErrVersionConflict) {
			c.JSON(409, map[string]interface{}{
				"code":    "VERSION_CONFLICT",
				"message": "collection has been modified by another user",
				"current_version": func() int {
					if col, _ := h.collectionService.GetByID(ctx, collectionID); col != nil {
						return col.Version
					}
					return 0
				}(),
			})
			return
		}
		c.InternalServerError("failed to update collection")
		return
	}

	h.hub.BroadcastCollectionUpdate(collection.WorkspaceID, collection.ID, userID, collection.Version)

	c.JSON(200, dto.CollectionResponse{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
		Data:        collection.Data,
		Version:     collection.Version,
		UpdatedBy:   collection.UpdatedBy,
	})
}

func (h *CollectionHandler) Delete(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	collectionID, err := uuid.Parse(c.Param("collectionId"))
	if err != nil {
		c.BadRequest("invalid collection id")
		return
	}

	ctx := context.Background()

	collection, err := h.collectionService.GetByID(ctx, collectionID)
	if err != nil {
		c.NotFound("collection not found")
		return
	}

	canModify, err := h.workspaceService.CanModify(ctx, collection.WorkspaceID, userID)
	if err != nil || !canModify {
		c.Forbidden("cannot delete this collection")
		return
	}

	if err := h.collectionService.Delete(ctx, collectionID); err != nil {
		c.InternalServerError("failed to delete collection")
		return
	}

	c.JSON(200, map[string]string{"message": "collection deleted"})
}
