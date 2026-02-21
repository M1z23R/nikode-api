package handlers

import (
	"context"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type UserHandler struct {
	userService UserServiceInterface
}

func NewUserHandler(userService UserServiceInterface) *UserHandler {
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

	_ = c.JSON(200, dto.UserResponse{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		AvatarURL:  user.AvatarURL,
		Provider:   user.Provider,
		GlobalRole: user.GlobalRole,
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

	_ = c.JSON(200, dto.UserResponse{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		AvatarURL:  user.AvatarURL,
		Provider:   user.Provider,
		GlobalRole: user.GlobalRole,
	})
}
