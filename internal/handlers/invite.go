package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type InviteHandler struct {
	teamService TeamServiceInterface
	userService UserServiceInterface
}

func NewInviteHandler(teamService TeamServiceInterface, userService UserServiceInterface) *InviteHandler {
	return &InviteHandler{
		teamService: teamService,
		userService: userService,
	}
}

func (h *InviteHandler) ViewInvite(c *drift.Context) {
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		h.renderError(c, "Invalid invite link")
		return
	}

	invite, err := h.teamService.GetInviteByID(context.Background(), inviteID)
	if err != nil {
		h.renderError(c, "Invite not found or has expired")
		return
	}

	if invite.Status != "pending" {
		h.renderMessage(c, "This invite has already been "+invite.Status)
		return
	}

	team, err := h.teamService.GetByID(context.Background(), invite.TeamID)
	if err != nil {
		h.renderError(c, "Team not found")
		return
	}

	inviter, _ := h.userService.GetByID(context.Background(), invite.InviterID)
	inviterName := "Someone"
	if inviter != nil {
		inviterName = inviter.Name
	}

	h.renderInvitePage(c, invite.ID.String(), team.Name, inviterName)
}

func (h *InviteHandler) AcceptInvite(c *drift.Context) {
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		h.renderError(c, "Invalid invite link")
		return
	}

	invite, err := h.teamService.GetInviteByID(context.Background(), inviteID)
	if err != nil {
		h.renderError(c, "Invite not found")
		return
	}

	if err := h.teamService.AcceptInvite(context.Background(), inviteID, invite.InviteeID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			h.renderError(c, "Invite not found or already processed")
			return
		}
		h.renderError(c, "Failed to accept invite")
		return
	}

	team, _ := h.teamService.GetByID(context.Background(), invite.TeamID)
	teamName := "the team"
	if team != nil {
		teamName = team.Name
	}

	h.renderMessage(c, fmt.Sprintf("You have joined %s!", teamName))
}

func (h *InviteHandler) DeclineInvite(c *drift.Context) {
	inviteID, err := uuid.Parse(c.Param("inviteId"))
	if err != nil {
		h.renderError(c, "Invalid invite link")
		return
	}

	invite, err := h.teamService.GetInviteByID(context.Background(), inviteID)
	if err != nil {
		h.renderError(c, "Invite not found")
		return
	}

	if err := h.teamService.DeclineInvite(context.Background(), inviteID, invite.InviteeID); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			h.renderError(c, "Invite not found or already processed")
			return
		}
		h.renderError(c, "Failed to decline invite")
		return
	}

	h.renderMessage(c, "Invite declined")
}

func (h *InviteHandler) renderInvitePage(c *drift.Context, inviteID, teamName, inviterName string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Team Invitation</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 400px; margin: 50px auto; padding: 20px; text-align: center; }
        h1 { color: #333; }
        p { color: #666; margin: 20px 0; }
        .team-name { font-weight: bold; color: #333; }
        .buttons { display: flex; gap: 10px; justify-content: center; margin-top: 30px; }
        button { padding: 12px 24px; font-size: 16px; border: none; border-radius: 6px; cursor: pointer; }
        .accept { background: #22c55e; color: white; }
        .accept:hover { background: #16a34a; }
        .decline { background: #e5e7eb; color: #333; }
        .decline:hover { background: #d1d5db; }
    </style>
</head>
<body>
    <h1>Team Invitation</h1>
    <p><strong>%s</strong> has invited you to join</p>
    <p class="team-name">%s</p>
    <div class="buttons">
        <form action="/invite/%s/accept" method="POST" style="display:inline;">
            <button type="submit" class="accept">Accept</button>
        </form>
        <form action="/invite/%s/decline" method="POST" style="display:inline;">
            <button type="submit" class="decline">Decline</button>
        </form>
    </div>
</body>
</html>`, inviterName, teamName, inviteID, inviteID)

	_ = c.HTML(200, html)
}

func (h *InviteHandler) renderMessage(c *drift.Context, message string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Team Invitation</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 400px; margin: 50px auto; padding: 20px; text-align: center; }
        h1 { color: #22c55e; }
        p { color: #666; }
    </style>
</head>
<body>
    <h1>%s</h1>
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
        body { font-family: system-ui, sans-serif; max-width: 400px; margin: 50px auto; padding: 20px; text-align: center; }
        h1 { color: #ef4444; }
        p { color: #666; }
    </style>
</head>
<body>
    <h1>Error</h1>
    <p>%s</p>
</body>
</html>`, message)

	_ = c.HTML(400, html)
}
