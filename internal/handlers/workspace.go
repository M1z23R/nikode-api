package handlers

import (
	"context"

	"github.com/m1z23r/drift/pkg/drift"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
)

type WorkspaceHandler struct {
	workspaceService *services.WorkspaceService
	teamService      *services.TeamService
}

func NewWorkspaceHandler(workspaceService *services.WorkspaceService, teamService *services.TeamService) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspaceService: workspaceService,
		teamService:      teamService,
	}
}

func (h *WorkspaceHandler) Create(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	var req dto.CreateWorkspaceRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	ctx := context.Background()

	if req.TeamID != nil {
		isMember, err := h.teamService.IsMember(ctx, *req.TeamID, userID)
		if err != nil || !isMember {
			c.Forbidden("not a member of this team")
			return
		}
	}

	workspace, err := h.workspaceService.Create(ctx, req.Name, userID, req.TeamID)
	if err != nil {
		c.InternalServerError("failed to create workspace")
		return
	}

	wsType := "personal"
	if workspace.TeamID != nil {
		wsType = "team"
	}

	_ = c.JSON(201, dto.WorkspaceResponse{
		ID:     workspace.ID,
		Name:   workspace.Name,
		UserID: workspace.UserID,
		TeamID: workspace.TeamID,
		Type:   wsType,
	})
}

func (h *WorkspaceHandler) List(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	workspaces, err := h.workspaceService.GetUserWorkspaces(context.Background(), userID)
	if err != nil {
		c.InternalServerError("failed to get workspaces")
		return
	}

	response := make([]dto.WorkspaceResponse, len(workspaces))
	for i, w := range workspaces {
		wsType := "personal"
		if w.TeamID != nil {
			wsType = "team"
		}
		response[i] = dto.WorkspaceResponse{
			ID:     w.ID,
			Name:   w.Name,
			UserID: w.UserID,
			TeamID: w.TeamID,
			Type:   wsType,
		}
	}

	_ = c.JSON(200, response)
}

func (h *WorkspaceHandler) Get(c *drift.Context) {
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

	workspace, err := h.workspaceService.GetByID(ctx, workspaceID)
	if err != nil {
		c.NotFound("workspace not found")
		return
	}

	wsType := "personal"
	if workspace.TeamID != nil {
		wsType = "team"
	}

	_ = c.JSON(200, dto.WorkspaceResponse{
		ID:     workspace.ID,
		Name:   workspace.Name,
		UserID: workspace.UserID,
		TeamID: workspace.TeamID,
		Type:   wsType,
	})
}

func (h *WorkspaceHandler) Update(c *drift.Context) {
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

	canModify, err := h.workspaceService.CanModify(ctx, workspaceID, userID)
	if err != nil || !canModify {
		c.Forbidden("cannot modify this workspace")
		return
	}

	var req dto.UpdateWorkspaceRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	workspace, err := h.workspaceService.Update(ctx, workspaceID, req.Name)
	if err != nil {
		c.InternalServerError("failed to update workspace")
		return
	}

	wsType := "personal"
	if workspace.TeamID != nil {
		wsType = "team"
	}

	_ = c.JSON(200, dto.WorkspaceResponse{
		ID:     workspace.ID,
		Name:   workspace.Name,
		UserID: workspace.UserID,
		TeamID: workspace.TeamID,
		Type:   wsType,
	})
}

func (h *WorkspaceHandler) Delete(c *drift.Context) {
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

	canModify, err := h.workspaceService.CanModify(ctx, workspaceID, userID)
	if err != nil || !canModify {
		c.Forbidden("cannot delete this workspace")
		return
	}

	if err := h.workspaceService.Delete(ctx, workspaceID); err != nil {
		c.InternalServerError("failed to delete workspace")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "workspace deleted"})
}
