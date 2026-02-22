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

const (
	tunnelPingInterval = 30 * time.Second
	tunnelWriteTimeout = 10 * time.Second
	tunnelReadTimeout  = 60 * time.Second
)

type TunnelMessage struct {
	Action string `json:"action"`

	// Register/Unregister
	Subdomain string `json:"subdomain,omitempty"`
	LocalPort int    `json:"local_port,omitempty"`

	// Response
	RequestID   string            `json:"request_id,omitempty"`
	StatusCode  int               `json:"status_code,omitempty"`
	RespHeaders map[string]string `json:"resp_headers,omitempty"`
	RespBody    string            `json:"resp_body,omitempty"`
	RespError   string            `json:"resp_error,omitempty"`
}

type TunnelHandler struct {
	hub        *hub.Hub
	jwtService *services.JWTService
}

func NewTunnelHandler(h *hub.Hub, jwtService *services.JWTService) *TunnelHandler {
	return &TunnelHandler{
		hub:        h,
		jwtService: jwtService,
	}
}

func (h *TunnelHandler) Connect(c *drift.Context) {
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

	// Upgrade to WebSocket
	conn, err := websocket.Upgrade(c)
	if err != nil {
		log.Printf("Tunnel WebSocket upgrade failed: %v", err)
		return
	}

	clientID := uuid.New().String()
	client := &hub.Client{
		ID:         clientID,
		UserID:     claims.UserID,
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	// Send connected message
	_ = conn.WriteJSON(map[string]string{
		"type":      "connected",
		"client_id": clientID,
	})

	done := make(chan struct{})

	// Write pump
	go func() {
		ticker := time.NewTicker(tunnelPingInterval)
		defer ticker.Stop()
		defer func() {
			if err := conn.Close(websocket.CloseNormalClosure, ""); err != nil {
				log.Printf("Tunnel WebSocket close error: %v", err)
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

	// Read pump (blocks until disconnect)
	func() {
		defer func() {
			close(done)
			close(client.Send)
			// Clean up all tunnels for this client
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
				h.handleResponse(msg)
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

func (h *TunnelHandler) handleRegister(conn *websocket.Conn, client *hub.Client, msg TunnelMessage, userID uuid.UUID) {
	subdomain := strings.ToLower(strings.TrimSpace(msg.Subdomain))

	if !isValidSubdomain(subdomain) {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid subdomain format (3-63 chars, alphanumeric and hyphens only)",
			"ref_action": "register",
		})
		return
	}

	if msg.LocalPort < 1 || msg.LocalPort > 65535 {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "invalid port (must be 1-65535)",
			"ref_action": "register",
		})
		return
	}

	err := h.hub.RegisterTunnel(subdomain, msg.LocalPort, client, userID)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    err.Error(),
			"ref_action": "register",
		})
		return
	}

	_ = conn.WriteJSON(map[string]any{
		"type":       "registered",
		"subdomain":  subdomain,
		"url":        fmt.Sprintf("https://%s.webhooks.nikode.dimitrije.dev", subdomain),
		"local_port": msg.LocalPort,
	})
}

func (h *TunnelHandler) handleUnregister(conn *websocket.Conn, client *hub.Client, msg TunnelMessage) {
	subdomain := strings.ToLower(strings.TrimSpace(msg.Subdomain))

	// Verify the client owns this tunnel
	info, ok := h.hub.GetTunnel(subdomain)
	if !ok || info.Client.ID != client.ID {
		_ = conn.WriteJSON(map[string]string{
			"type":       "error",
			"message":    "tunnel not found or not owned by you",
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

func (h *TunnelHandler) handleResponse(msg TunnelMessage) {
	h.hub.HandleTunnelResponse(&hub.TunnelResponse{
		ID:         msg.RequestID,
		StatusCode: msg.StatusCode,
		Headers:    msg.RespHeaders,
		Body:       msg.RespBody,
		Error:      msg.RespError,
	})
}

func (h *TunnelHandler) handleCheck(conn *websocket.Conn, msg TunnelMessage) {
	subdomain := strings.ToLower(strings.TrimSpace(msg.Subdomain))
	available := h.hub.IsSubdomainAvailable(subdomain)

	_ = conn.WriteJSON(map[string]any{
		"type":      "check_result",
		"subdomain": subdomain,
		"available": available,
	})
}

func isValidSubdomain(s string) bool {
	if len(s) < 3 || len(s) > 63 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return s[0] != '-' && s[len(s)-1] != '-'
}
