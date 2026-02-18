package hub

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

type Event struct {
	Type        string      `json:"type"`
	WorkspaceID *uuid.UUID  `json:"workspace_id,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}

type CollectionCreatedData struct {
	CollectionID uuid.UUID `json:"collection_id"`
	Name         string    `json:"name"`
	Version      int       `json:"version"`
	CreatedBy    uuid.UUID `json:"created_by"`
}

type CollectionUpdatedData struct {
	CollectionID uuid.UUID `json:"collection_id"`
	Name         string    `json:"name"`
	Version      int       `json:"version"`
	UpdatedBy    uuid.UUID `json:"updated_by"`
}

type CollectionDeletedData struct {
	CollectionID uuid.UUID `json:"collection_id"`
	DeletedBy    uuid.UUID `json:"deleted_by"`
}

type WorkspaceUpdatedData struct {
	Name      string    `json:"name"`
	UpdatedBy uuid.UUID `json:"updated_by"`
}

type MemberJoinedData struct {
	UserID        uuid.UUID `json:"user_id"`
	UserName      string    `json:"user_name"`
	UserAvatarURL *string   `json:"user_avatar_url,omitempty"`
}

type MemberLeftData struct {
	UserID uuid.UUID `json:"user_id"`
}

type OnlineUser struct {
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

type PresenceUpdateData struct {
	OnlineUsers []OnlineUser `json:"online_users"`
}

type Client struct {
	ID         string
	UserID     uuid.UUID
	UserName   string
	AvatarURL  *string
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
				// Collect workspaces before removing
				workspaces := make([]uuid.UUID, 0, len(client.Workspaces))
				for wsID := range client.Workspaces {
					workspaces = append(workspaces, wsID)
				}
				delete(h.clients, client.ID)
				close(client.Send)
				h.mu.Unlock()

				// Broadcast presence updates for all workspaces this client was in
				for _, wsID := range workspaces {
					h.broadcastPresence(wsID)
				}
			} else {
				h.mu.Unlock()
			}

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
	if client, ok := h.clients[clientID]; ok {
		client.Workspaces[workspaceID] = true
	}
	h.mu.Unlock()

	h.broadcastPresence(workspaceID)
}

func (h *Hub) UnsubscribeFromWorkspace(clientID string, workspaceID uuid.UUID) {
	h.mu.Lock()
	if client, ok := h.clients[clientID]; ok {
		delete(client.Workspaces, workspaceID)
	}
	h.mu.Unlock()

	h.broadcastPresence(workspaceID)
}

func (h *Hub) BroadcastCollectionCreate(workspaceID, collectionID, createdBy uuid.UUID, name string, version int) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type:        "collection_created",
			WorkspaceID: &workspaceID,
			Data: CollectionCreatedData{
				CollectionID: collectionID,
				Name:         name,
				Version:      version,
				CreatedBy:    createdBy,
			},
		},
	}
}

func (h *Hub) BroadcastCollectionUpdate(workspaceID, collectionID, updatedBy uuid.UUID, name string, version int) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type:        "collection_updated",
			WorkspaceID: &workspaceID,
			Data: CollectionUpdatedData{
				CollectionID: collectionID,
				Name:         name,
				Version:      version,
				UpdatedBy:    updatedBy,
			},
		},
	}
}

func (h *Hub) BroadcastCollectionDelete(workspaceID, collectionID, deletedBy uuid.UUID) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type:        "collection_deleted",
			WorkspaceID: &workspaceID,
			Data: CollectionDeletedData{
				CollectionID: collectionID,
				DeletedBy:    deletedBy,
			},
		},
	}
}

func (h *Hub) BroadcastWorkspaceUpdate(workspaceID, updatedBy uuid.UUID, name string) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type:        "workspace_updated",
			WorkspaceID: &workspaceID,
			Data: WorkspaceUpdatedData{
				Name:      name,
				UpdatedBy: updatedBy,
			},
		},
	}
}

func (h *Hub) BroadcastMemberJoined(workspaceID, userID uuid.UUID, userName string, avatarURL *string) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type:        "member_joined",
			WorkspaceID: &workspaceID,
			Data: MemberJoinedData{
				UserID:        userID,
				UserName:      userName,
				UserAvatarURL: avatarURL,
			},
		},
	}
}

func (h *Hub) BroadcastMemberLeft(workspaceID, userID uuid.UUID) {
	h.broadcast <- &WorkspaceMessage{
		WorkspaceID: workspaceID,
		Event: Event{
			Type:        "member_left",
			WorkspaceID: &workspaceID,
			Data: MemberLeftData{
				UserID: userID,
			},
		},
	}
}

// broadcastPresence computes the current online users for a workspace and broadcasts it.
func (h *Hub) broadcastPresence(workspaceID uuid.UUID) {
	h.mu.RLock()
	seen := make(map[uuid.UUID]bool)
	var onlineUsers []OnlineUser
	for _, client := range h.clients {
		if client.Workspaces[workspaceID] && !seen[client.UserID] {
			seen[client.UserID] = true
			onlineUsers = append(onlineUsers, OnlineUser{
				UserID:    client.UserID,
				UserName:  client.UserName,
				AvatarURL: client.AvatarURL,
			})
		}
	}
	h.mu.RUnlock()

	if onlineUsers == nil {
		onlineUsers = []OnlineUser{}
	}

	event := Event{
		Type:        "presence_update",
		WorkspaceID: &workspaceID,
		Data: PresenceUpdateData{
			OnlineUsers: onlineUsers,
		},
	}

	data, _ := json.Marshal(event)

	h.mu.RLock()
	for _, client := range h.clients {
		if client.Workspaces[workspaceID] {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
	h.mu.RUnlock()
}
