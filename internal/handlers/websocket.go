package handlers

import (
	"log"

	"github.com/m1z23r/drift/pkg/drift"
	"github.com/m1z23r/drift/pkg/websocket"
)

type WebSocketHandler struct{}

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{}
}

func (h *WebSocketHandler) Connect(c *drift.Context) {
	conn, err := websocket.Upgrade(c)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(websocket.CloseNormalClosure, ""); err != nil {
			log.Printf("WebSocket close error: %v", err)
		}
	}()

	if err := conn.WriteJSON(map[string]string{
		"type":    "connected",
		"message": "WebSocket connection established",
	}); err != nil {
		return
	}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		switch msgType {
		case websocket.TextMessage:
			if err := conn.WriteText(string(data)); err != nil {
				return
			}
		case websocket.BinaryMessage:
			if err := conn.WriteBinary(data); err != nil {
				return
			}
		}
	}
}
