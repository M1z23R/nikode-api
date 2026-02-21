package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type WorkspaceHandler struct {
	workspaceService WorkspaceServiceInterface
	userService      UserServiceInterface
	emailService     EmailServiceInterface
	hub              HubInterface
	baseURL          string
}

func NewWorkspaceHandler(workspaceService WorkspaceServiceInterface, userService UserServiceInterface, emailService EmailServiceInterface, hub HubInterface, baseURL string) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspaceService: workspaceService,
		userService:      userService,
		emailService:     emailService,
		hub:              hub,
		baseURL:          baseURL,
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

	workspace, err := h.workspaceService.Create(context.Background(), req.Name, userID)
	if err != nil {
		log.Printf("failed to create workspace: %v", err)
		c.InternalServerError("failed to create workspace")
		return
	}

	_ = c.JSON(201, dto.WorkspaceResponse{
		ID:      workspace.ID,
		Name:    workspace.Name,
		OwnerID: workspace.OwnerID,
		Role:    "owner",
	})
}

func (h *WorkspaceHandler) List(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	workspaces, roles, err := h.workspaceService.GetUserWorkspaces(context.Background(), userID)
	if err != nil {
		c.InternalServerError("failed to get workspaces")
		return
	}

	response := make([]dto.WorkspaceResponse, len(workspaces))
	for i, w := range workspaces {
		response[i] = dto.WorkspaceResponse{
			ID:      w.ID,
			Name:    w.Name,
			OwnerID: w.OwnerID,
			Role:    roles[i],
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

	role := "member"
	if workspace.OwnerID == userID {
		role = "owner"
	}

	_ = c.JSON(200, dto.WorkspaceResponse{
		ID:      workspace.ID,
		Name:    workspace.Name,
		OwnerID: workspace.OwnerID,
		Role:    role,
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

	h.hub.BroadcastWorkspaceUpdate(workspaceID, userID, workspace.Name)

	_ = c.JSON(200, dto.WorkspaceResponse{
		ID:      workspace.ID,
		Name:    workspace.Name,
		OwnerID: workspace.OwnerID,
		Role:    "owner",
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

	// Get members before deleting so we can notify them
	members, _ := h.workspaceService.GetMembers(ctx, workspaceID)

	if err := h.workspaceService.Delete(ctx, workspaceID); err != nil {
		c.InternalServerError("failed to delete workspace")
		return
	}

	// Notify all members their workspace list changed
	for _, member := range members {
		h.hub.BroadcastToUser(member.UserID, "workspaces_changed", hub.WorkspacesChangedData{
			Reason:      "workspace_deleted",
			WorkspaceID: workspaceID,
		})
	}

	_ = c.JSON(200, map[string]string{"message": "workspace deleted"})
}

func (h *WorkspaceHandler) GetMembers(c *drift.Context) {
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

	isMember, err := h.workspaceService.IsMember(context.Background(), workspaceID, userID)
	if err != nil || !isMember {
		c.NotFound("workspace not found")
		return
	}

	members, err := h.workspaceService.GetMembers(context.Background(), workspaceID)
	if err != nil {
		c.InternalServerError("failed to get members")
		return
	}

	response := make([]dto.WorkspaceMemberResponse, len(members))
	for i, m := range members {
		response[i] = dto.WorkspaceMemberResponse{
			ID:     m.ID,
			UserID: m.UserID,
			Role:   m.Role,
			User: dto.UserResponse{
				ID:         m.User.ID,
				Email:      m.User.Email,
				Name:       m.User.Name,
				AvatarURL:  m.User.AvatarURL,
				Provider:   m.User.Provider,
				GlobalRole: m.User.GlobalRole,
			},
		}
	}

	_ = c.JSON(200, response)
}

func (h *WorkspaceHandler) InviteMember(c *drift.Context) {
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
	if err != nil || !isOwner {
		c.Forbidden("only owner can invite members")
		return
	}

	var req dto.InviteMemberRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Email == "" {
		c.BadRequest("email is required")
		return
	}

	invitee, err := h.userService.GetByEmail(context.Background(), req.Email)
	if err != nil {
		c.NotFound("user with this email not found")
		return
	}

	invite, err := h.workspaceService.CreateInvite(context.Background(), workspaceID, userID, invitee.ID)
	if err != nil {
		if errors.Is(err, services.ErrAlreadyMember) {
			c.BadRequest("user is already a workspace member")
			return
		}
		c.InternalServerError("failed to create invite")
		return
	}

	workspace, _ := h.workspaceService.GetByID(context.Background(), workspaceID)
	inviter, _ := h.userService.GetByID(context.Background(), userID)
	if workspace != nil && inviter != nil {
		inviteURL := fmt.Sprintf("%s/invite/%s", h.baseURL, invite.ID)
		_ = h.emailService.SendWorkspaceInvite(invitee.Email, workspace.Name, inviter.Name, inviteURL)
	}

	// Notify invitee they have a new pending invite
	h.hub.BroadcastToUser(invitee.ID, "workspaces_changed", hub.WorkspacesChangedData{
		Reason:      "invite_received",
		WorkspaceID: workspaceID,
	})

	_ = c.JSON(201, dto.WorkspaceInviteResponse{
		ID:          invite.ID,
		WorkspaceID: invite.WorkspaceID,
		Status:      invite.Status,
		CreatedAt:   invite.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *WorkspaceHandler) RemoveMember(c *drift.Context) {
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

	memberID, err := uuid.Parse(c.Param("memberId"))
	if err != nil {
		c.BadRequest("invalid member id")
		return
	}

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil || !isOwner {
		c.Forbidden("only owner can remove members")
		return
	}

	if memberID == userID {
		c.BadRequest("cannot remove yourself as owner")
		return
	}

	if err := h.workspaceService.RemoveMember(context.Background(), workspaceID, memberID); err != nil {
		if errors.Is(err, services.ErrCannotRemoveOwner) {
			c.BadRequest("cannot remove workspace owner")
			return
		}
		if errors.Is(err, services.ErrMemberNotFound) {
			c.NotFound("member not found")
			return
		}
		c.InternalServerError("failed to remove member")
		return
	}

	h.hub.BroadcastMemberLeft(workspaceID, memberID)

	// Notify removed member their workspace list changed
	h.hub.BroadcastToUser(memberID, "workspaces_changed", hub.WorkspacesChangedData{
		Reason:      "removed_from_workspace",
		WorkspaceID: workspaceID,
	})

	_ = c.JSON(200, map[string]string{"message": "member removed"})
}

func (h *WorkspaceHandler) LeaveWorkspace(c *drift.Context) {
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

	if err := h.workspaceService.RemoveMember(context.Background(), workspaceID, userID); err != nil {
		if errors.Is(err, services.ErrCannotRemoveOwner) {
			c.BadRequest("owner cannot leave workspace, transfer ownership or delete it")
			return
		}
		if errors.Is(err, services.ErrMemberNotFound) {
			c.NotFound("workspace not found or not a member")
			return
		}
		c.InternalServerError("failed to leave workspace")
		return
	}

	h.hub.BroadcastMemberLeft(workspaceID, userID)

	_ = c.JSON(200, map[string]string{"message": "left workspace"})
}

func (h *WorkspaceHandler) GetWorkspaceInvites(c *drift.Context) {
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
	if err != nil || !isOwner {
		c.Forbidden("only owner can view invites")
		return
	}

	invites, err := h.workspaceService.GetWorkspacePendingInvites(context.Background(), workspaceID)
	if err != nil {
		c.InternalServerError("failed to get invites")
		return
	}

	response := make([]dto.WorkspaceInviteResponse, len(invites))
	for i, inv := range invites {
		response[i] = dto.WorkspaceInviteResponse{
			ID:          inv.ID,
			WorkspaceID: inv.WorkspaceID,
			Status:      inv.Status,
			CreatedAt:   inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if inv.Invitee != nil {
			response[i].Invitee = &dto.UserResponse{
				ID:         inv.Invitee.ID,
				Email:      inv.Invitee.Email,
				Name:       inv.Invitee.Name,
				AvatarURL:  inv.Invitee.AvatarURL,
				Provider:   inv.Invitee.Provider,
				GlobalRole: inv.Invitee.GlobalRole,
			}
		}
	}

	_ = c.JSON(200, response)
}

func (h *WorkspaceHandler) CancelInvite(c *drift.Context) {
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

	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		c.BadRequest("invalid invite id")
		return
	}

	isOwner, err := h.workspaceService.IsOwner(context.Background(), workspaceID, userID)
	if err != nil || !isOwner {
		c.Forbidden("only owner can cancel invites")
		return
	}

	if err := h.workspaceService.CancelInvite(context.Background(), inviteID, workspaceID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			c.NotFound("invite not found")
			return
		}
		c.InternalServerError("failed to cancel invite")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "invite cancelled"})
}

func (h *WorkspaceHandler) GetMyInvites(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	invites, err := h.workspaceService.GetUserPendingInvites(context.Background(), userID)
	if err != nil {
		c.InternalServerError("failed to get invites")
		return
	}

	response := make([]dto.WorkspaceInviteResponse, len(invites))
	for i, inv := range invites {
		response[i] = dto.WorkspaceInviteResponse{
			ID:          inv.ID,
			WorkspaceID: inv.WorkspaceID,
			Status:      inv.Status,
			CreatedAt:   inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if inv.Workspace != nil {
			response[i].Workspace = &dto.WorkspaceResponse{
				ID:      inv.Workspace.ID,
				Name:    inv.Workspace.Name,
				OwnerID: inv.Workspace.OwnerID,
			}
		}
		if inv.Inviter != nil {
			response[i].Inviter = &dto.UserResponse{
				ID:         inv.Inviter.ID,
				Email:      inv.Inviter.Email,
				Name:       inv.Inviter.Name,
				AvatarURL:  inv.Inviter.AvatarURL,
				Provider:   inv.Inviter.Provider,
				GlobalRole: inv.Inviter.GlobalRole,
			}
		}
	}

	_ = c.JSON(200, response)
}

func (h *WorkspaceHandler) AcceptInvite(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		c.BadRequest("invalid invite id")
		return
	}

	if err := h.workspaceService.AcceptInvite(context.Background(), inviteID, userID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			c.NotFound("invite not found")
			return
		}
		c.InternalServerError("failed to accept invite")
		return
	}

	// Look up invite for workspace ID, and user for name/avatar
	invite, _ := h.workspaceService.GetInviteByID(context.Background(), inviteID)
	user, _ := h.userService.GetByID(context.Background(), userID)
	if invite != nil && user != nil {
		h.hub.BroadcastMemberJoined(invite.WorkspaceID, userID, user.Name, user.AvatarURL)
	}

	// Notify accepting user their workspace list changed
	if invite != nil {
		h.hub.BroadcastToUser(userID, "workspaces_changed", hub.WorkspacesChangedData{
			Reason:      "invite_accepted",
			WorkspaceID: invite.WorkspaceID,
		})
	}

	_ = c.JSON(200, map[string]string{"message": "invite accepted"})
}

func (h *WorkspaceHandler) DeclineInvite(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		c.BadRequest("invalid invite id")
		return
	}

	if err := h.workspaceService.DeclineInvite(context.Background(), inviteID, userID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			c.NotFound("invite not found")
			return
		}
		c.InternalServerError("failed to decline invite")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "invite declined"})
}
