package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
	"github.com/m1z23r/drift/pkg/websocket"
)

type WebhookWSHandler struct {
	hub        *hub.Hub
	jwtService *services.JWTService
}

func NewWebhookWSHandler(h *hub.Hub, jwtService *services.JWTService) *WebhookWSHandler {
	return &WebhookWSHandler{
		hub:        h,
		jwtService: jwtService,
	}
}

func (h *WebhookWSHandler) Connect(c *drift.Context) {
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

	conn, err := websocket.Upgrade(c)
	if err != nil {
		log.Printf("Webhook WebSocket upgrade failed: %v", err)
		return
	}

	clientID := uuid.New().String()
	client := &hub.Client{
		ID:         clientID,
		UserID:     claims.UserID,
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	_ = conn.WriteJSON(map[string]string{
		"type":      "connected",
		"client_id": clientID,
	})

	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(tunnelPingInterval)
		defer ticker.Stop()
		defer func() {
			if err := conn.Close(websocket.CloseNormalClosure, ""); err != nil {
				log.Printf("Webhook WebSocket close error: %v", err)
			}
		}()

		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(tunnelWriteTimeout))
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

	func() {
		defer func() {
			close(done)
			close(client.Send)
			h.hub.UnregisterClientTunnels(clientID)
		}()

		for {
			_ = conn.SetReadDeadline(time.Now().Add(tunnelReadTimeout))
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if msgType != websocket.TextMessage {
				continue
			}

			var msg TunnelMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = conn.WriteJSON(map[string]string{
					"type":    "error",
					"message": "invalid message format",
				})
				continue
			}

			switch msg.Action {
			case "register":
				h.handleRegister(conn, client, msg, claims.UserID)
			case "unregister":
				h.handleUnregister(conn, client, msg)
			case "response":
				h.hub.HandleTunnelResponse(&hub.TunnelResponse{
					ID:         msg.RequestID,
					StatusCode: msg.StatusCode,
					Headers:    msg.RespHeaders,
					Body:       msg.RespBody,
					Error:      msg.RespError,
				})
			case "check":
				h.handleCheck(conn, msg)
			case "ping":
				_ = conn.WriteJSON(map[string]string{"type": "pong"})
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

func (h *WebhookWSHandler) handleRegister(conn *websocket.Conn, client *hub.Client, msg TunnelMessage, userID uuid.UUID) {
	subdomain := strings.ToLower(strings.TrimSpace(msg.Subdomain))

	if !isValidSubdomain(subdomain) {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid subdomain format (3-63 chars, alphanumeric and hyphens only)",
			"ref_action": "register",
		})
		return
	}

	if err := h.hub.RegisterWebhook(subdomain, client, userID); err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    err.Error(),
			"ref_action": "register",
		})
		return
	}

	_ = conn.WriteJSON(map[string]any{
		"type":      "registered",
		"subdomain": subdomain,
		"url":       fmt.Sprintf("https://%s.webhooks.nikode.dimitrije.dev", subdomain),
	})
}

func (h *WebhookWSHandler) handleUnregister(conn *websocket.Conn, client *hub.Client, msg TunnelMessage) {
	subdomain := strings.ToLower(strings.TrimSpace(msg.Subdomain))

	info, ok := h.hub.GetTunnel(subdomain)
	if !ok || info.Client.ID != client.ID {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "webhook not found or not owned by you",
			"ref_action": "unregister",
		})
		return
	}

	h.hub.UnregisterTunnel(subdomain)

	_ = conn.WriteJSON(map[string]string{
		"type":      "unregistered",
		"subdomain": subdomain,
	})
}

func (h *WebhookWSHandler) handleCheck(conn *websocket.Conn, msg TunnelMessage) {
	subdomain := strings.ToLower(strings.TrimSpace(msg.Subdomain))
	available := h.hub.IsSubdomainAvailable(subdomain)

	_ = conn.WriteJSON(map[string]any{
		"type":      "check_result",
		"subdomain": subdomain,
		"available": available,
	})
}
