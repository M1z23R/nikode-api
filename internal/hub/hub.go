package hub

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Chat constants
const (
	DefaultChatHistorySize = 100
	MaxMessageLength       = 4000
	RateLimitMessages      = 10
	RateLimitWindow        = 10 * time.Second
)

// Chat errors
var (
	ErrRateLimited      = errors.New("rate limited: too many messages")
	ErrMessageTooLong   = errors.New("message exceeds maximum length")
	ErrNotSubscribed    = errors.New("not subscribed to workspace")
	ErrInvalidPublicKey = errors.New("invalid public key")
)

type Event struct {
	Type        string     `json:"type"`
	WorkspaceID *uuid.UUID `json:"workspace_id,omitempty"`
	Data        any        `json:"data,omitempty"`
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
	PublicKey  string // E2E encryption public key
}

// ChatMessage represents an encrypted chat message stored in memory
type ChatMessage struct {
	ID        string    `json:"id"`
	SenderID  uuid.UUID `json:"sender_id"`
	SenderName string   `json:"sender_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	Content   string    `json:"content"`   // Encrypted ciphertext
	Encrypted bool      `json:"encrypted"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatMessageData is the JSON representation sent to clients
type ChatMessageData struct {
	ID         string    `json:"id"`
	SenderID   uuid.UUID `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	AvatarURL  *string   `json:"avatar_url,omitempty"`
	Content    string    `json:"content"`
	Encrypted  bool      `json:"encrypted"`
	Timestamp  string    `json:"timestamp"`
}

// PublicKeyInfo holds a user's public key for key exchange
type PublicKeyInfo struct {
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	PublicKey string    `json:"public_key"`
}

// MessageRingBuffer is a fixed-size circular buffer for chat messages
type MessageRingBuffer struct {
	messages []ChatMessage
	head     int
	size     int
	capacity int
	mu       sync.RWMutex
}

// NewMessageRingBuffer creates a new ring buffer with the given capacity
func NewMessageRingBuffer(capacity int) *MessageRingBuffer {
	return &MessageRingBuffer{
		messages: make([]ChatMessage, capacity),
		capacity: capacity,
	}
}

// Add adds a message to the ring buffer
func (rb *MessageRingBuffer) Add(msg ChatMessage) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.messages[rb.head] = msg
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetRecent returns the most recent n messages (oldest first)
func (rb *MessageRingBuffer) GetRecent(n int) []ChatMessage {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.size {
		n = rb.size
	}
	if n <= 0 {
		return []ChatMessage{}
	}

	result := make([]ChatMessage, n)
	start := (rb.head - n + rb.capacity) % rb.capacity
	for i := 0; i < n; i++ {
		result[i] = rb.messages[(start+i)%rb.capacity]
	}
	return result
}

// ChatRateLimiter implements a sliding window rate limiter
type ChatRateLimiter struct {
	userTimestamps map[uuid.UUID][]time.Time
	maxMessages    int
	windowSize     time.Duration
	mu             sync.Mutex
}

// NewChatRateLimiter creates a new rate limiter
func NewChatRateLimiter(maxMessages int, windowSize time.Duration) *ChatRateLimiter {
	return &ChatRateLimiter{
		userTimestamps: make(map[uuid.UUID][]time.Time),
		maxMessages:    maxMessages,
		windowSize:     windowSize,
	}
}

// Allow checks if a user can send a message and records the attempt
func (rl *ChatRateLimiter) Allow(userID uuid.UUID) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.windowSize)

	// Filter out old timestamps
	timestamps := rl.userTimestamps[userID]
	valid := make([]time.Time, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}

	if len(valid) >= rl.maxMessages {
		rl.userTimestamps[userID] = valid
		return false
	}

	rl.userTimestamps[userID] = append(valid, now)
	return true
}

// ChatBroadcastMessage is used for broadcasting chat messages
type ChatBroadcastMessage struct {
	WorkspaceID uuid.UUID
	Message     ChatMessage
}

type Hub struct {
	clients       map[string]*Client
	register      chan *Client
	unregister    chan *Client
	broadcast     chan *WorkspaceMessage
	userBroadcast chan *UserMessage
	mu            sync.RWMutex

	// Chat
	chatBroadcast   chan *ChatBroadcastMessage
	chatHistory     map[uuid.UUID]*MessageRingBuffer // workspaceID -> buffer
	chatRateLimiter *ChatRateLimiter
	chatMu          sync.RWMutex

	// E2E Key Exchange
	publicKeys map[uuid.UUID]string // userID -> publicKey
	keysMu     sync.RWMutex
}

type WorkspaceMessage struct {
	WorkspaceID uuid.UUID
	Event       Event
}

type UserMessage struct {
	UserID uuid.UUID
	Event  Event
}

type WorkspacesChangedData struct {
	Reason      string    `json:"reason"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

func NewHub() *Hub {
	return &Hub{
		clients:         make(map[string]*Client),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		broadcast:       make(chan *WorkspaceMessage, 256),
		userBroadcast:   make(chan *UserMessage, 256),
		chatBroadcast:   make(chan *ChatBroadcastMessage, 256),
		chatHistory:     make(map[uuid.UUID]*MessageRingBuffer),
		chatRateLimiter: NewChatRateLimiter(RateLimitMessages, RateLimitWindow),
		publicKeys:      make(map[uuid.UUID]string),
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

		case msg := <-h.userBroadcast:
			h.mu.RLock()
			data, _ := json.Marshal(msg.Event)
			for _, client := range h.clients {
				if client.UserID == msg.UserID {
					select {
					case client.Send <- data:
					default:
						// Client buffer full, skip
					}
				}
			}
			h.mu.RUnlock()

		case chatMsg := <-h.chatBroadcast:
			h.mu.RLock()
			event := Event{
				Type:        "chat_message",
				WorkspaceID: &chatMsg.WorkspaceID,
				Data: ChatMessageData{
					ID:         chatMsg.Message.ID,
					SenderID:   chatMsg.Message.SenderID,
					SenderName: chatMsg.Message.SenderName,
					AvatarURL:  chatMsg.Message.AvatarURL,
					Content:    chatMsg.Message.Content,
					Encrypted:  chatMsg.Message.Encrypted,
					Timestamp:  chatMsg.Message.Timestamp.UTC().Format(time.RFC3339),
				},
			}
			data, _ := json.Marshal(event)
			for _, client := range h.clients {
				if client.Workspaces[chatMsg.WorkspaceID] {
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
	var newClient *Client
	if client, ok := h.clients[clientID]; ok {
		client.Workspaces[workspaceID] = true
		newClient = client
	}
	h.mu.Unlock()

	// Trigger key exchange if new client has a public key
	if newClient != nil && newClient.PublicKey != "" {
		h.triggerKeyExchange(workspaceID, newClient)
	}

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

func (h *Hub) BroadcastToUser(userID uuid.UUID, eventType string, data any) {
	h.userBroadcast <- &UserMessage{
		UserID: userID,
		Event: Event{
			Type: eventType,
			Data: data,
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

// SetPublicKey registers a client's public key for E2E encryption
func (h *Hub) SetPublicKey(clientID string, publicKey string) {
	h.mu.Lock()
	client, ok := h.clients[clientID]
	if ok {
		client.PublicKey = publicKey
	}
	h.mu.Unlock()

	if ok {
		h.keysMu.Lock()
		h.publicKeys[client.UserID] = publicKey
		h.keysMu.Unlock()

		// Trigger key exchange for all workspaces this client is already subscribed to.
		// This handles the race condition where subscribe arrives before set_public_key.
		h.mu.RLock()
		workspaces := make([]uuid.UUID, 0, len(client.Workspaces))
		for wsID := range client.Workspaces {
			workspaces = append(workspaces, wsID)
		}
		h.mu.RUnlock()

		for _, wsID := range workspaces {
			h.triggerKeyExchange(wsID, client)
		}
	}
}

// GetPublicKey returns a user's public key
func (h *Hub) GetPublicKey(userID uuid.UUID) string {
	h.keysMu.RLock()
	defer h.keysMu.RUnlock()
	return h.publicKeys[userID]
}

// GetWorkspacePublicKeys returns public keys of all members subscribed to a workspace
func (h *Hub) GetWorkspacePublicKeys(workspaceID uuid.UUID) []PublicKeyInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := make(map[uuid.UUID]bool)
	var keys []PublicKeyInfo

	for _, client := range h.clients {
		if client.Workspaces[workspaceID] && !seen[client.UserID] && client.PublicKey != "" {
			seen[client.UserID] = true
			keys = append(keys, PublicKeyInfo{
				UserID:    client.UserID,
				UserName:  client.UserName,
				PublicKey: client.PublicKey,
			})
		}
	}

	if keys == nil {
		keys = []PublicKeyInfo{}
	}
	return keys
}

// triggerKeyExchange notifies existing members about a new subscriber and sends workspace keys to the new subscriber
func (h *Hub) triggerKeyExchange(workspaceID uuid.UUID, newClient *Client) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Collect existing members with public keys
	var existingMembers []PublicKeyInfo
	for _, client := range h.clients {
		if client.Workspaces[workspaceID] && client.ID != newClient.ID && client.PublicKey != "" {
			existingMembers = append(existingMembers, PublicKeyInfo{
				UserID:    client.UserID,
				UserName:  client.UserName,
				PublicKey: client.PublicKey,
			})
		}
	}

	// Notify existing members about the new subscriber
	if newClient.PublicKey != "" {
		keyExchangeEvent := Event{
			Type:        "key_exchange_needed",
			WorkspaceID: &workspaceID,
			Data: map[string]any{
				"user_id":    newClient.UserID,
				"user_name":  newClient.UserName,
				"public_key": newClient.PublicKey,
			},
		}
		data, _ := json.Marshal(keyExchangeEvent)

		for _, client := range h.clients {
			if client.Workspaces[workspaceID] && client.ID != newClient.ID {
				select {
				case client.Send <- data:
				default:
				}
			}
		}
	}

	// Send existing members' keys to the new subscriber
	if existingMembers == nil {
		existingMembers = []PublicKeyInfo{}
	}
	workspaceKeysEvent := Event{
		Type:        "workspace_keys",
		WorkspaceID: &workspaceID,
		Data: map[string]any{
			"members": existingMembers,
		},
	}
	data, _ := json.Marshal(workspaceKeysEvent)
	select {
	case newClient.Send <- data:
	default:
	}
}

// RelayEncryptedKey relays an encrypted symmetric key from one user to another
func (h *Hub) RelayEncryptedKey(targetUserID uuid.UUID, fromUserID uuid.UUID, workspaceID uuid.UUID, encryptedKey string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	event := Event{
		Type:        "workspace_key",
		WorkspaceID: &workspaceID,
		Data: map[string]any{
			"from_user_id":  fromUserID,
			"encrypted_key": encryptedKey,
		},
	}
	data, _ := json.Marshal(event)

	for _, client := range h.clients {
		if client.UserID == targetUserID {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
}

// SendChatMessage sends a chat message to a workspace
func (h *Hub) SendChatMessage(workspaceID, senderID uuid.UUID, senderName string, avatarURL *string, content string, encrypted bool) (*ChatMessage, error) {
	// Validate message length
	if len(content) > MaxMessageLength {
		return nil, ErrMessageTooLong
	}

	// Check rate limit
	if !h.chatRateLimiter.Allow(senderID) {
		return nil, ErrRateLimited
	}

	// Create message
	msg := ChatMessage{
		ID:         uuid.New().String(),
		SenderID:   senderID,
		SenderName: senderName,
		AvatarURL:  avatarURL,
		Content:    content,
		Encrypted:  encrypted,
		Timestamp:  time.Now().UTC(),
	}

	// Store in history
	h.chatMu.Lock()
	buffer, ok := h.chatHistory[workspaceID]
	if !ok {
		buffer = NewMessageRingBuffer(DefaultChatHistorySize)
		h.chatHistory[workspaceID] = buffer
	}
	buffer.Add(msg)
	h.chatMu.Unlock()

	// Broadcast to workspace
	h.chatBroadcast <- &ChatBroadcastMessage{
		WorkspaceID: workspaceID,
		Message:     msg,
	}

	return &msg, nil
}

// GetChatHistory returns the most recent chat messages for a workspace
func (h *Hub) GetChatHistory(workspaceID uuid.UUID, limit int) []ChatMessage {
	if limit <= 0 {
		limit = DefaultChatHistorySize
	}

	h.chatMu.RLock()
	buffer, ok := h.chatHistory[workspaceID]
	h.chatMu.RUnlock()

	if !ok {
		return []ChatMessage{}
	}

	return buffer.GetRecent(limit)
}

// IsSubscribedToWorkspace checks if a client is subscribed to a workspace
func (h *Hub) IsSubscribedToWorkspace(clientID string, workspaceID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, ok := h.clients[clientID]
	if !ok {
		return false
	}
	return client.Workspaces[workspaceID]
}
