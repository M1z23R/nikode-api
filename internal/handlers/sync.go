package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
	"github.com/m1z23r/drift/pkg/websocket"
)

const (
	syncPingInterval = 30 * time.Second
	syncWriteTimeout = 10 * time.Second
	syncReadTimeout  = 60 * time.Second
)

type ClientMessage struct {
	Action      string `json:"action"`
	WorkspaceID string `json:"workspace_id,omitempty"`

	// Chat
	Content   string `json:"content,omitempty"`
	Encrypted bool   `json:"encrypted,omitempty"`
	Limit     int    `json:"limit,omitempty"`

	// Key exchange
	PublicKey    string `json:"public_key,omitempty"`
	TargetUserID string `json:"target_user_id,omitempty"`
	EncryptedKey string `json:"encrypted_key,omitempty"`
}

type SyncHandler struct {
	hub              HubInterface
	workspaceService WorkspaceServiceInterface
	userService      UserServiceInterface
	jwtService       *services.JWTService
}

func NewSyncHandler(hub HubInterface, workspaceService WorkspaceServiceInterface, userService UserServiceInterface, jwtService *services.JWTService) *SyncHandler {
	return &SyncHandler{
		hub:              hub,
		workspaceService: workspaceService,
		userService:      userService,
		jwtService:       jwtService,
	}
}

func (h *SyncHandler) Connect(c *drift.Context) {
	// Extract and validate JWT before upgrading
	token := c.QueryParam("token")
	if token == "" {
		c.Unauthorized("token is required")
		return
	}

	claims, err := h.jwtService.ValidateAccessToken(token)
	if err != nil {
		c.Unauthorized("invalid token")
		return
	}

	// Look up user for name/avatar
	user, err := h.userService.GetByID(context.Background(), claims.UserID)
	if err != nil {
		c.Unauthorized("user not found")
		return
	}

	// Upgrade to WebSocket
	conn, err := websocket.Upgrade(c)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	clientID := uuid.New().String()
	client := &hub.Client{
		ID:         clientID,
		UserID:     claims.UserID,
		UserName:   user.Name,
		AvatarURL:  user.AvatarURL,
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	h.hub.Register(client)

	// Send connected message
	_ = conn.WriteJSON(map[string]string{
		"type":      "connected",
		"client_id": clientID,
	})

	done := make(chan struct{})

	// Write pump
	go func() {
		ticker := time.NewTicker(syncPingInterval)
		defer ticker.Stop()
		defer func() {
			if err := conn.Close(websocket.CloseNormalClosure, ""); err != nil {
				log.Printf("WebSocket close error: %v", err)
			}
		}()

		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(syncWriteTimeout))
				if err := conn.WriteText(string(msg)); err != nil {
					return
				}
			case <-ticker.C:
				if err := conn.Ping(nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Read pump (blocks until disconnect)
	func() {
		defer func() {
			close(done)
			h.hub.Unregister(client)
		}()

		for {
			_ = conn.SetReadDeadline(time.Now().Add(syncReadTimeout))
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if msgType != websocket.TextMessage {
				continue
			}

			var msg ClientMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = conn.WriteJSON(map[string]string{
					"type":    "error",
					"message": "invalid message format",
				})
				continue
			}

			switch msg.Action {
			case "subscribe":
				h.handleSubscribe(conn, client, msg)
			case "unsubscribe":
				h.handleUnsubscribe(conn, client, msg)
			case "ping":
				_ = conn.WriteJSON(map[string]string{"type": "pong"})
			case "set_public_key":
				h.handleSetPublicKey(conn, client, msg)
			case "send_chat":
				h.handleSendChat(conn, client, msg)
			case "get_chat_history":
				h.handleGetChatHistory(conn, client, msg)
			case "share_workspace_key":
				h.handleShareWorkspaceKey(conn, client, msg)
			case "key_ready":
				h.handleKeyReady(conn, client, msg)
			default:
				_ = conn.WriteJSON(map[string]string{
					"type":       "error",
					"message":    "unknown action",
					"ref_action": msg.Action,
				})
			}
		}
	}()
}

func (h *SyncHandler) handleSubscribe(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	workspaceID, err := uuid.Parse(msg.WorkspaceID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid workspace_id",
			"ref_action": "subscribe",
		})
		return
	}

	canAccess, err := h.workspaceService.CanAccess(context.Background(), workspaceID, client.UserID)
	if err != nil || !canAccess {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "workspace not found or access denied",
			"ref_action": "subscribe",
		})
		return
	}

	h.hub.SubscribeToWorkspace(client.ID, workspaceID)

	_ = conn.WriteJSON(map[string]string{
		"type":         "subscribed",
		"workspace_id": workspaceID.String(),
	})
}

func (h *SyncHandler) handleUnsubscribe(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	workspaceID, err := uuid.Parse(msg.WorkspaceID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid workspace_id",
			"ref_action": "unsubscribe",
		})
		return
	}

	h.hub.UnsubscribeFromWorkspace(client.ID, workspaceID)

	_ = conn.WriteJSON(map[string]string{
		"type":         "unsubscribed",
		"workspace_id": workspaceID.String(),
	})
}

func (h *SyncHandler) handleSetPublicKey(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	if msg.PublicKey == "" {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "public_key is required",
			"ref_action": "set_public_key",
		})
		return
	}

	h.hub.SetPublicKey(client.ID, msg.PublicKey)

	_ = conn.WriteJSON(map[string]string{
		"type": "public_key_set",
	})
}

func (h *SyncHandler) handleSendChat(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	workspaceID, err := uuid.Parse(msg.WorkspaceID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid workspace_id",
			"ref_action": "send_chat",
		})
		return
	}

	// Check if subscribed
	if !h.hub.IsSubscribedToWorkspace(client.ID, workspaceID) {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "not subscribed to workspace",
			"ref_action": "send_chat",
		})
		return
	}

	if msg.Content == "" {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "content is required",
			"ref_action": "send_chat",
		})
		return
	}

	chatMsg, err := h.hub.SendChatMessage(workspaceID, client.UserID, client.UserName, client.AvatarURL, msg.Content, msg.Encrypted)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    err.Error(),
			"ref_action": "send_chat",
		})
		return
	}

	_ = conn.WriteJSON(map[string]string{
		"type":       "chat_sent",
		"message_id": chatMsg.ID,
	})
}

func (h *SyncHandler) handleGetChatHistory(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	workspaceID, err := uuid.Parse(msg.WorkspaceID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid workspace_id",
			"ref_action": "get_chat_history",
		})
		return
	}

	// Check if subscribed
	if !h.hub.IsSubscribedToWorkspace(client.ID, workspaceID) {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "not subscribed to workspace",
			"ref_action": "get_chat_history",
		})
		return
	}

	limit := msg.Limit
	if limit <= 0 {
		limit = 50
	}

	messages := h.hub.GetChatHistory(workspaceID, limit)

	// Convert to ChatMessageData
	messageData := make([]hub.ChatMessageData, len(messages))
	for i, m := range messages {
		messageData[i] = hub.ChatMessageData{
			ID:         m.ID,
			SenderID:   m.SenderID,
			SenderName: m.SenderName,
			AvatarURL:  m.AvatarURL,
			Content:    m.Content,
			Encrypted:  m.Encrypted,
			Timestamp:  m.Timestamp.UTC().Format(time.RFC3339),
		}
	}

	_ = conn.WriteJSON(map[string]any{
		"type":         "chat_history",
		"workspace_id": workspaceID.String(),
		"messages":     messageData,
	})
}

func (h *SyncHandler) handleShareWorkspaceKey(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	workspaceID, err := uuid.Parse(msg.WorkspaceID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid workspace_id",
			"ref_action": "share_workspace_key",
		})
		return
	}

	targetUserID, err := uuid.Parse(msg.TargetUserID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid target_user_id",
			"ref_action": "share_workspace_key",
		})
		return
	}

	if msg.EncryptedKey == "" {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "encrypted_key is required",
			"ref_action": "share_workspace_key",
		})
		return
	}

	// Check if subscribed
	if !h.hub.IsSubscribedToWorkspace(client.ID, workspaceID) {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "not subscribed to workspace",
			"ref_action": "share_workspace_key",
		})
		return
	}

	h.hub.RelayEncryptedKey(targetUserID, client.UserID, workspaceID, msg.EncryptedKey)

	_ = conn.WriteJSON(map[string]string{
		"type": "workspace_key_shared",
	})
}

func (h *SyncHandler) handleKeyReady(conn *websocket.Conn, client *hub.Client, msg ClientMessage) {
	workspaceID, err := uuid.Parse(msg.WorkspaceID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid workspace_id",
			"ref_action": "key_ready",
		})
		return
	}

	// Check if subscribed
	if !h.hub.IsSubscribedToWorkspace(client.ID, workspaceID) {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "not subscribed to workspace",
			"ref_action": "key_ready",
		})
		return
	}

	h.hub.MarkKeyReady(client.UserID, workspaceID)

	_ = conn.WriteJSON(map[string]string{
		"type": "key_ready_confirmed",
	})
}
