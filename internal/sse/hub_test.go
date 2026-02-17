package sse

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	assert.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.NotNil(t, hub.broadcast)
}

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)

	// Wait for registration to process
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.clients[client.ID]
	hub.mu.RUnlock()

	assert.True(t, exists)
}

func TestHub_UnregisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.clients[client.ID]
	hub.mu.RUnlock()

	assert.False(t, exists)
}

func TestHub_UnregisterClient_ClosesSendChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	// Send channel should be closed
	_, ok := <-client.Send
	assert.False(t, ok)
}

func TestHub_SubscribeToWorkspace(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}
	workspaceID := uuid.New()

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeToWorkspace(client.ID, workspaceID)

	hub.mu.RLock()
	isSubscribed := client.Workspaces[workspaceID]
	hub.mu.RUnlock()

	assert.True(t, isSubscribed)
}

func TestHub_UnsubscribeFromWorkspace(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.UnsubscribeFromWorkspace(client.ID, workspaceID)

	hub.mu.RLock()
	isSubscribed := client.Workspaces[workspaceID]
	hub.mu.RUnlock()

	assert.False(t, isSubscribed)
}

func TestHub_BroadcastCollectionUpdate_ToSubscribedClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	collectionID := uuid.New()
	updatedBy := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionUpdate(workspaceID, collectionID, updatedBy, 2)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "collection_updated", event.Type)

		// Verify event data
		dataBytes, _ := json.Marshal(event.Data)
		var updateEvent CollectionUpdatedEvent
		err = json.Unmarshal(dataBytes, &updateEvent)
		require.NoError(t, err)

		assert.Equal(t, collectionID, updateEvent.CollectionID)
		assert.Equal(t, workspaceID, updateEvent.WorkspaceID)
		assert.Equal(t, updatedBy, updateEvent.UpdatedBy)
		assert.Equal(t, 2, updateEvent.Version)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive message")
	}
}

func TestHub_BroadcastCollectionUpdate_NotToUnsubscribedClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{otherWorkspaceID: true}, // Subscribed to different workspace
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionUpdate(workspaceID, uuid.New(), uuid.New(), 1)

	select {
	case <-client.Send:
		t.Fatal("should not have received message")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message received
	}
}

func TestHub_BroadcastCollectionUpdate_ToMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()

	client1 := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}
	client2 := &Client{
		ID:         "client-2",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}
	client3 := &Client{
		ID:         "client-3",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{uuid.New(): true}, // Different workspace
		Send:       make(chan []byte, 256),
	}

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionUpdate(workspaceID, uuid.New(), uuid.New(), 1)

	// Client 1 and 2 should receive, client 3 should not
	receivedCount := 0

	select {
	case <-client1.Send:
		receivedCount++
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case <-client2.Send:
		receivedCount++
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case <-client3.Send:
		t.Fatal("client3 should not receive message")
	case <-time.After(50 * time.Millisecond):
	}

	assert.Equal(t, 2, receivedCount)
}

func TestHub_BroadcastCollectionUpdate_FullBufferDropped(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()

	// Create client with small buffer
	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 1), // Very small buffer
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	client.Send <- []byte("fill")

	// This should not panic - message should be dropped
	hub.BroadcastCollectionUpdate(workspaceID, uuid.New(), uuid.New(), 1)
	time.Sleep(10 * time.Millisecond)

	// Drain the buffer
	<-client.Send

	// Should not receive the dropped message
	select {
	case <-client.Send:
		t.Fatal("should not receive dropped message")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestHub_SubscribeToWorkspace_NonexistentClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Should not panic when client doesn't exist
	hub.SubscribeToWorkspace("nonexistent", uuid.New())
}

func TestHub_UnsubscribeFromWorkspace_NonexistentClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Should not panic when client doesn't exist
	hub.UnsubscribeFromWorkspace("nonexistent", uuid.New())
}

func TestHub_UnregisterNonexistentClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		ID:         "nonexistent",
		UserID:     uuid.New(),
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	// Should not panic
	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)
}

func TestHub_MultipleWorkspaceSubscriptions(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspace1 := uuid.New()
	workspace2 := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeToWorkspace(client.ID, workspace1)
	hub.SubscribeToWorkspace(client.ID, workspace2)

	hub.mu.RLock()
	assert.True(t, client.Workspaces[workspace1])
	assert.True(t, client.Workspaces[workspace2])
	hub.mu.RUnlock()
}
