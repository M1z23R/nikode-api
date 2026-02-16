package sse

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type CollectionUpdatedEvent struct {
	CollectionID uuid.UUID `json:"collection_id"`
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	Version      int       `json:"version"`
	UpdatedBy    uuid.UUID `json:"updated_by"`
}

type Client struct {
	ID         string
	UserID     uuid.UUID
	Workspaces map[uuid.UUID]bool
	Send       chan []byte
}

type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *WorkspaceMessage
	mu         sync.RWMutex
}

type WorkspaceMessage struct {
	WorkspaceID uuid.UUID
	Event       Event
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *WorkspaceMessage, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			data, _ := json.Marshal(msg.Event)
			for _, client := range h.clients {
				if client.Workspaces[msg.WorkspaceID] {
					select {
					case client.Send <- data:
					default:
						// Client buffer full, skip
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) SubscribeToWorkspace(clientID string, workspaceID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client, ok := h.clients[clientID]; ok {
		client.Workspaces[workspaceID] = true
	}
}

func (h *Hub) UnsubscribeFromWorkspace(clientID string, workspaceID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client, ok := h.clients[clientID]; ok {
		delete(client.Workspaces, workspaceID)
	}
}

func (h *Hub) BroadcastCollectionUpdate(workspaceID, collectionID, updatedBy uuid.UUID, version int) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type: "collection_updated",
			Data: CollectionUpdatedEvent{
				CollectionID: collectionID,
				WorkspaceID:  workspaceID,
				Version:      version,
				UpdatedBy:    updatedBy,
			},
		},
	}
}
