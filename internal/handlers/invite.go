package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type InviteHandler struct {
	workspaceService WorkspaceServiceInterface
	hub              HubInterface
}

func NewInviteHandler(workspaceService WorkspaceServiceInterface, hub HubInterface) *InviteHandler {
	return &InviteHandler{
		workspaceService: workspaceService,
		hub:              hub,
	}
}

func (h *InviteHandler) ViewInvite(c *drift.Context) {
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		h.renderError(c, "Invalid invite link")
		return
	}

	invite, err := h.workspaceService.GetInviteWithDetails(context.Background(), inviteID)
	if err != nil {
		h.renderError(c, "Invite not found or has expired")
		return
	}

	if invite.Status != "pending" {
		h.renderMessage(c, "This invite has already been "+invite.Status)
		return
	}

	workspaceName := "Unknown Workspace"
	if invite.Workspace != nil {
		workspaceName = invite.Workspace.Name
	}

	inviterName := "Someone"
	if invite.Inviter != nil {
		inviterName = invite.Inviter.Name
	}

	h.renderInvitePage(c, invite.ID.String(), workspaceName, inviterName)
}

func (h *InviteHandler) AcceptInvite(c *drift.Context) {
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		h.renderError(c, "Invalid invite link")
		return
	}

	invite, err := h.workspaceService.GetInviteByID(context.Background(), inviteID)
	if err != nil {
		h.renderError(c, "Invite not found")
		return
	}

	if err := h.workspaceService.AcceptInvite(context.Background(), inviteID, invite.InviteeID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			h.renderError(c, "Invite not found or already processed")
			return
		}
		h.renderError(c, "Failed to accept invite")
		return
	}

	workspace, _ := h.workspaceService.GetByID(context.Background(), invite.WorkspaceID)
	workspaceName := "the workspace"
	if workspace != nil {
		workspaceName = workspace.Name
	}

	h.hub.BroadcastToUser(invite.InviteeID, "workspaces_changed", hub.WorkspacesChangedData{
		Reason:      "invite_accepted",
		WorkspaceID: invite.WorkspaceID,
	})

	h.renderMessage(c, fmt.Sprintf("You have joined %s!", workspaceName))
}

func (h *InviteHandler) DeclineInvite(c *drift.Context) {
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		h.renderError(c, "Invalid invite link")
		return
	}

	invite, err := h.workspaceService.GetInviteByID(context.Background(), inviteID)
	if err != nil {
		h.renderError(c, "Invite not found")
		return
	}

	if err := h.workspaceService.DeclineInvite(context.Background(), inviteID, invite.InviteeID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			h.renderError(c, "Invite not found or already processed")
			return
		}
		h.renderError(c, "Failed to decline invite")
		return
	}

	h.renderMessage(c, "Invite declined")
}

func (h *InviteHandler) renderInvitePage(c *drift.Context, inviteID, workspaceName, inviterName string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Workspace Invitation</title>
    <style>
        * { box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; background: #f9fafb; color: #374151; margin: 0; padding: 40px 20px; min-height: 100vh; }
        .container { max-width: 400px; margin: 0 auto; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 40px 32px; text-align: center; }
        .icon { margin-bottom: 24px; }
        .icon svg { width: 48px; height: 48px; }
        h1 { font-size: 20px; font-weight: 600; color: #111827; margin: 0 0 8px 0; }
        .subtitle { color: #6b7280; font-size: 14px; margin: 0 0 24px 0; }
        .workspace-name { font-size: 18px; font-weight: 600; color: #111827; padding: 16px; background: #f3f4f6; border-radius: 6px; margin-bottom: 32px; }
        .buttons { display: flex; gap: 12px; justify-content: center; }
        button { padding: 10px 20px; font-size: 14px; font-weight: 500; border: none; border-radius: 6px; cursor: pointer; transition: background 0.15s; }
        .accept { background: #374151; color: #fff; }
        .accept:hover { background: #1f2937; }
        .decline { background: #fff; color: #374151; border: 1px solid #d1d5db; }
        .decline:hover { background: #f3f4f6; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg width="512" height="512" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
                <rect x="0" y="0" width="512" height="512" rx="80" ry="80" fill="#374151"/>
                <text x="256" y="380" font-family="Arial, Helvetica, sans-serif" font-size="360" font-weight="bold" fill="#f3f4f6" text-anchor="middle">N</text>
            </svg>
        </div>
        <h1>Workspace Invitation</h1>
        <p class="subtitle"><strong>%s</strong> has invited you to join</p>
        <div class="workspace-name">%s</div>
        <div class="buttons">
            <form action="/invite/%s/decline" method="POST" style="display:inline;">
                <button type="submit" class="decline">Decline</button>
            </form>
            <form action="/invite/%s/accept" method="POST" style="display:inline;">
                <button type="submit" class="accept">Accept</button>
            </form>
        </div>
    </div>
</body>
</html>`, inviterName, workspaceName, inviteID, inviteID)

	_ = c.HTML(200, html)
}

func (h *InviteHandler) renderMessage(c *drift.Context, message string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Workspace Invitation</title>
    <style>
        * { box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; background: #f9fafb; color: #374151; margin: 0; padding: 40px 20px; min-height: 100vh; }
        .container { max-width: 400px; margin: 0 auto; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 40px 32px; text-align: center; }
        .icon { margin-bottom: 24px; }
        .icon svg { width: 48px; height: 48px; }
        h1 { font-size: 18px; font-weight: 600; color: #111827; margin: 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg width="512" height="512" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
                <rect x="0" y="0" width="512" height="512" rx="80" ry="80" fill="#374151"/>
                <text x="256" y="380" font-family="Arial, Helvetica, sans-serif" font-size="360" font-weight="bold" fill="#f3f4f6" text-anchor="middle">N</text>
            </svg>
        </div>
        <h1>%s</h1>
    </div>
</body>
</html>`, message)

	_ = c.HTML(200, html)
}

func (h *InviteHandler) renderError(c *drift.Context, message string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error</title>
    <style>
        * { box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; background: #f9fafb; color: #374151; margin: 0; padding: 40px 20px; min-height: 100vh; }
        .container { max-width: 400px; margin: 0 auto; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 40px 32px; text-align: center; }
        .icon { margin-bottom: 24px; }
        .icon svg { width: 48px; height: 48px; }
        h1 { font-size: 18px; font-weight: 600; color: #991b1b; margin: 0 0 8px 0; }
        p { color: #6b7280; font-size: 14px; margin: 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg width="512" height="512" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
                <rect x="0" y="0" width="512" height="512" rx="80" ry="80" fill="#374151"/>
                <text x="256" y="380" font-family="Arial, Helvetica, sans-serif" font-size="360" font-weight="bold" fill="#f3f4f6" text-anchor="middle">N</text>
            </svg>
        </div>
        <h1>Error</h1>
        <p>%s</p>
    </div>
</body>
</html>`, message)

	_ = c.HTML(400, html)
}
