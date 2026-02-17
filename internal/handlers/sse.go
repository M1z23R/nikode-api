package handlers

import (
	"context"
	"fmt"

	"github.com/m1z23r/drift/pkg/drift"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/internal/sse"
	"github.com/google/uuid"
)

type SSEHandler struct {
	hub              *sse.Hub
	workspaceService *services.WorkspaceService
}

func NewSSEHandler(hub *sse.Hub, workspaceService *services.WorkspaceService) *SSEHandler {
	return &SSEHandler{
		hub:              hub,
		workspaceService: workspaceService,
	}
}

func (h *SSEHandler) Connect(c *drift.Context) {
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

	sseCtx := c.SSE()

	clientID := uuid.New().String()
	client := &sse.Client{
		ID:         clientID,
		UserID:     userID,
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	h.hub.Register(client)
	defer h.hub.Unregister(client)

	if err := sseCtx.SendJSON(map[string]string{
		"type":      "connected",
		"client_id": clientID,
	}, "system", ""); err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		<-c.Request.Context().Done()
		close(done)
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				return
			}
			if err := sseCtx.Send(string(msg), "message", ""); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func (h *SSEHandler) Subscribe(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	clientID := c.Param("clientId")
	if clientID == "" {
		c.BadRequest("client_id is required")
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

	h.hub.SubscribeToWorkspace(clientID, workspaceID)

	_ = c.JSON(200, map[string]string{
		"message": fmt.Sprintf("subscribed to workspace %s", workspaceID),
	})
}

func (h *SSEHandler) Unsubscribe(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	clientID := c.Param("clientId")
	if clientID == "" {
		c.BadRequest("client_id is required")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	h.hub.UnsubscribeFromWorkspace(clientID, workspaceID)

	_ = c.JSON(200, map[string]string{
		"message": fmt.Sprintf("unsubscribed from workspace %s", workspaceID),
	})
}
