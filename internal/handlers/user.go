package handlers

import (
	"context"

	"github.com/m1z23r/drift/pkg/drift"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) GetMe(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	user, err := h.userService.GetByID(context.Background(), userID)
	if err != nil {
		c.NotFound("user not found")
		return
	}

	c.JSON(200, dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		Provider:  user.Provider,
	})
}

func (h *UserHandler) UpdateMe(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	var req dto.UpdateUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	user, err := h.userService.Update(context.Background(), userID, req.Name)
	if err != nil {
		c.InternalServerError("failed to update user")
		return
	}

	c.JSON(200, dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		Provider:  user.Provider,
	})
}
