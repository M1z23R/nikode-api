package handlers

import (
	"context"
	"errors"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type TeamHandler struct {
	teamService *services.TeamService
	userService *services.UserService
}

func NewTeamHandler(teamService *services.TeamService, userService *services.UserService) *TeamHandler {
	return &TeamHandler{
		teamService: teamService,
		userService: userService,
	}
}

func (h *TeamHandler) Create(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	var req dto.CreateTeamRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	team, err := h.teamService.Create(context.Background(), req.Name, userID)
	if err != nil {
		c.InternalServerError("failed to create team")
		return
	}

	_ = c.JSON(201, dto.TeamResponse{
		ID:      team.ID,
		Name:    team.Name,
		OwnerID: team.OwnerID,
		Role:    "owner",
	})
}

func (h *TeamHandler) List(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teams, roles, err := h.teamService.GetUserTeams(context.Background(), userID)
	if err != nil {
		c.InternalServerError("failed to get teams")
		return
	}

	response := make([]dto.TeamResponse, len(teams))
	for i, team := range teams {
		response[i] = dto.TeamResponse{
			ID:      team.ID,
			Name:    team.Name,
			OwnerID: team.OwnerID,
			Role:    roles[i],
		}
	}

	_ = c.JSON(200, response)
}

func (h *TeamHandler) Get(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	isMember, err := h.teamService.IsMember(context.Background(), teamID, userID)
	if err != nil || !isMember {
		c.NotFound("team not found")
		return
	}

	team, err := h.teamService.GetByID(context.Background(), teamID)
	if err != nil {
		c.NotFound("team not found")
		return
	}

	role := "member"
	if team.OwnerID == userID {
		role = "owner"
	}

	_ = c.JSON(200, dto.TeamResponse{
		ID:      team.ID,
		Name:    team.Name,
		OwnerID: team.OwnerID,
		Role:    role,
	})
}

func (h *TeamHandler) Update(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	isOwner, err := h.teamService.IsOwner(context.Background(), teamID, userID)
	if err != nil || !isOwner {
		c.Forbidden("only owner can update team")
		return
	}

	var req dto.UpdateTeamRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	team, err := h.teamService.Update(context.Background(), teamID, req.Name)
	if err != nil {
		c.InternalServerError("failed to update team")
		return
	}

	_ = c.JSON(200, dto.TeamResponse{
		ID:      team.ID,
		Name:    team.Name,
		OwnerID: team.OwnerID,
		Role:    "owner",
	})
}

func (h *TeamHandler) Delete(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	isOwner, err := h.teamService.IsOwner(context.Background(), teamID, userID)
	if err != nil || !isOwner {
		c.Forbidden("only owner can delete team")
		return
	}

	if err := h.teamService.Delete(context.Background(), teamID); err != nil {
		c.InternalServerError("failed to delete team")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "team deleted"})
}

func (h *TeamHandler) GetMembers(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	isMember, err := h.teamService.IsMember(context.Background(), teamID, userID)
	if err != nil || !isMember {
		c.NotFound("team not found")
		return
	}

	members, err := h.teamService.GetMembers(context.Background(), teamID)
	if err != nil {
		c.InternalServerError("failed to get members")
		return
	}

	response := make([]dto.TeamMemberResponse, len(members))
	for i, m := range members {
		response[i] = dto.TeamMemberResponse{
			ID:     m.ID,
			UserID: m.UserID,
			Role:   m.Role,
			User: dto.UserResponse{
				ID:        m.User.ID,
				Email:     m.User.Email,
				Name:      m.User.Name,
				AvatarURL: m.User.AvatarURL,
				Provider:  m.User.Provider,
			},
		}
	}

	_ = c.JSON(200, response)
}

func (h *TeamHandler) InviteMember(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	isOwner, err := h.teamService.IsOwner(context.Background(), teamID, userID)
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

	if err := h.teamService.AddMember(context.Background(), teamID, invitee.ID); err != nil {
		c.InternalServerError("failed to add member")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "member added"})
}

func (h *TeamHandler) RemoveMember(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	memberID, err := uuid.Parse(c.Param("memberId"))
	if err != nil {
		c.BadRequest("invalid member id")
		return
	}

	isOwner, err := h.teamService.IsOwner(context.Background(), teamID, userID)
	if err != nil || !isOwner {
		c.Forbidden("only owner can remove members")
		return
	}

	if memberID == userID {
		c.BadRequest("cannot remove yourself as owner")
		return
	}

	if err := h.teamService.RemoveMember(context.Background(), teamID, memberID); err != nil {
		if errors.Is(err, services.ErrCannotRemoveOwner) {
			c.BadRequest("cannot remove team owner")
			return
		}
		if errors.Is(err, services.ErrMemberNotFound) {
			c.NotFound("member not found")
			return
		}
		c.InternalServerError("failed to remove member")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "member removed"})
}

func (h *TeamHandler) LeaveTeam(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.BadRequest("invalid team id")
		return
	}

	if err := h.teamService.RemoveMember(context.Background(), teamID, userID); err != nil {
		if errors.Is(err, services.ErrCannotRemoveOwner) {
			c.BadRequest("owner cannot leave team, transfer ownership or delete it")
			return
		}
		if errors.Is(err, services.ErrMemberNotFound) {
			c.NotFound("team not found or not a member")
			return
		}
		c.InternalServerError("failed to leave team")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "left team"})
}
