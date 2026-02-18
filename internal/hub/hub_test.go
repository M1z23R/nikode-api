package hub

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
		UserName:   "Test User",
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
		UserName:   "Test User",
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
		UserName:   "Test User",
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
		UserName:   "Test User",
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

	// Drain presence update
	drainChannel(client.Send, 100*time.Millisecond)
}

func TestHub_UnsubscribeFromWorkspace(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
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
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionUpdate(workspaceID, collectionID, updatedBy, "My Collection", 2)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "collection_updated", event.Type)
		assert.Equal(t, workspaceID, *event.WorkspaceID)

		dataBytes, _ := json.Marshal(event.Data)
		var updateData CollectionUpdatedData
		err = json.Unmarshal(dataBytes, &updateData)
		require.NoError(t, err)

		assert.Equal(t, collectionID, updateData.CollectionID)
		assert.Equal(t, "My Collection", updateData.Name)
		assert.Equal(t, updatedBy, updateData.UpdatedBy)
		assert.Equal(t, 2, updateData.Version)

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
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{otherWorkspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionUpdate(workspaceID, uuid.New(), uuid.New(), "Col", 1)

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
		UserName:   "User 1",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}
	client2 := &Client{
		ID:         "client-2",
		UserID:     uuid.New(),
		UserName:   "User 2",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}
	client3 := &Client{
		ID:         "client-3",
		UserID:     uuid.New(),
		UserName:   "User 3",
		Workspaces: map[uuid.UUID]bool{uuid.New(): true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionUpdate(workspaceID, uuid.New(), uuid.New(), "Col", 1)

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

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 1),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	client.Send <- []byte("fill")

	// This should not panic - message should be dropped
	hub.BroadcastCollectionUpdate(workspaceID, uuid.New(), uuid.New(), "Col", 1)
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
		UserName:   "Test User",
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
		UserName:   "Test User",
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

	// Drain presence updates
	drainChannel(client.Send, 100*time.Millisecond)
}

func TestHub_BroadcastCollectionCreate(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	collectionID := uuid.New()
	createdBy := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionCreate(workspaceID, collectionID, createdBy, "New Collection", 1)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "collection_created", event.Type)
		assert.Equal(t, workspaceID, *event.WorkspaceID)

		dataBytes, _ := json.Marshal(event.Data)
		var createData CollectionCreatedData
		err = json.Unmarshal(dataBytes, &createData)
		require.NoError(t, err)

		assert.Equal(t, collectionID, createData.CollectionID)
		assert.Equal(t, "New Collection", createData.Name)
		assert.Equal(t, createdBy, createData.CreatedBy)
		assert.Equal(t, 1, createData.Version)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive message")
	}
}

func TestHub_BroadcastCollectionDelete(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	collectionID := uuid.New()
	deletedBy := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastCollectionDelete(workspaceID, collectionID, deletedBy)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "collection_deleted", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var deleteData CollectionDeletedData
		err = json.Unmarshal(dataBytes, &deleteData)
		require.NoError(t, err)

		assert.Equal(t, collectionID, deleteData.CollectionID)
		assert.Equal(t, deletedBy, deleteData.DeletedBy)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive message")
	}
}

func TestHub_BroadcastWorkspaceUpdate(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	updatedBy := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastWorkspaceUpdate(workspaceID, updatedBy, "New Name")

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "workspace_updated", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var wsData WorkspaceUpdatedData
		err = json.Unmarshal(dataBytes, &wsData)
		require.NoError(t, err)

		assert.Equal(t, "New Name", wsData.Name)
		assert.Equal(t, updatedBy, wsData.UpdatedBy)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive message")
	}
}

func TestHub_BroadcastMemberJoined(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	userID := uuid.New()
	avatar := "https://example.com/avatar.png"

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastMemberJoined(workspaceID, userID, "New Member", &avatar)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "member_joined", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var memberData MemberJoinedData
		err = json.Unmarshal(dataBytes, &memberData)
		require.NoError(t, err)

		assert.Equal(t, userID, memberData.UserID)
		assert.Equal(t, "New Member", memberData.UserName)
		assert.Equal(t, &avatar, memberData.UserAvatarURL)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive message")
	}
}

func TestHub_BroadcastMemberLeft(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	userID := uuid.New()

	client := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastMemberLeft(workspaceID, userID)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "member_left", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var memberData MemberLeftData
		err = json.Unmarshal(dataBytes, &memberData)
		require.NoError(t, err)

		assert.Equal(t, userID, memberData.UserID)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive message")
	}
}

func TestHub_PresenceUpdate_OnSubscribe(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	userID := uuid.New()
	avatar := "https://example.com/avatar.png"

	client := &Client{
		ID:         "client-1",
		UserID:     userID,
		UserName:   "Test User",
		AvatarURL:  &avatar,
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeToWorkspace(client.ID, workspaceID)

	select {
	case msg := <-client.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "presence_update", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var presenceData PresenceUpdateData
		err = json.Unmarshal(dataBytes, &presenceData)
		require.NoError(t, err)

		assert.Len(t, presenceData.OnlineUsers, 1)
		assert.Equal(t, userID, presenceData.OnlineUsers[0].UserID)
		assert.Equal(t, "Test User", presenceData.OnlineUsers[0].UserName)
		assert.Equal(t, &avatar, presenceData.OnlineUsers[0].AvatarURL)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive presence update")
	}
}

func TestHub_PresenceUpdate_DeduplicatesByUserID(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()
	userID := uuid.New()

	// Two clients with same UserID (e.g., multiple browser tabs)
	client1 := &Client{
		ID:         "client-1",
		UserID:     userID,
		UserName:   "Test User",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}
	client2 := &Client{
		ID:         "client-2",
		UserID:     userID,
		UserName:   "Test User",
		Workspaces: make(map[uuid.UUID]bool),
		Send:       make(chan []byte, 256),
	}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeToWorkspace(client2.ID, workspaceID)

	// Client1 should get the presence update
	select {
	case msg := <-client1.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "presence_update", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var presenceData PresenceUpdateData
		err = json.Unmarshal(dataBytes, &presenceData)
		require.NoError(t, err)

		// Should be deduplicated to 1 user
		assert.Len(t, presenceData.OnlineUsers, 1)
		assert.Equal(t, userID, presenceData.OnlineUsers[0].UserID)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive presence update")
	}
}

func TestHub_PresenceUpdate_OnUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	workspaceID := uuid.New()

	client1 := &Client{
		ID:         "client-1",
		UserID:     uuid.New(),
		UserName:   "User 1",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}
	client2 := &Client{
		ID:         "client-2",
		UserID:     uuid.New(),
		UserName:   "User 2",
		Workspaces: map[uuid.UUID]bool{workspaceID: true},
		Send:       make(chan []byte, 256),
	}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	// Unregister client2, client1 should get presence update
	hub.Unregister(client2)

	select {
	case msg := <-client1.Send:
		var event Event
		err := json.Unmarshal(msg, &event)
		require.NoError(t, err)

		assert.Equal(t, "presence_update", event.Type)

		dataBytes, _ := json.Marshal(event.Data)
		var presenceData PresenceUpdateData
		err = json.Unmarshal(dataBytes, &presenceData)
		require.NoError(t, err)

		// Only client1's user should remain
		assert.Len(t, presenceData.OnlineUsers, 1)
		assert.Equal(t, client1.UserID, presenceData.OnlineUsers[0].UserID)

	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive presence update after unregister")
	}
}

// drainChannel drains all messages from a channel within a timeout.
func drainChannel(ch chan []byte, timeout time.Duration) {
	for {
		select {
		case <-ch:
		case <-time.After(timeout):
			return
		}
	}
}
