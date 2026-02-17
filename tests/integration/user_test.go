package integration

import (
	"context"
	"testing"

	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_Integration_FindOrCreateFromOAuth_CreateNew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	svc := services.NewUserService(tdb.DB)
	ctx := context.Background()

	info := &oauth.UserInfo{
		Email:     "newuser@example.com",
		Name:      "New User",
		AvatarURL: "https://example.com/avatar.png",
		ID:        "github-12345",
		Provider:  "github",
	}

	user, err := svc.FindOrCreateFromOAuth(ctx, info)

	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, info.Email, user.Email)
	assert.Equal(t, info.Name, user.Name)
	assert.Equal(t, info.Provider, user.Provider)
	assert.Equal(t, info.ID, user.ProviderID)
}

func TestUserService_Integration_FindOrCreateFromOAuth_FindExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	svc := services.NewUserService(tdb.DB)
	ctx := context.Background()

	info := &oauth.UserInfo{
		Email:     "existinguser@example.com",
		Name:      "Existing User",
		AvatarURL: "https://example.com/avatar.png",
		ID:        "github-99999",
		Provider:  "github",
	}

	// Create user first
	user1, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	// Find same user
	user2, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	assert.Equal(t, user1.ID, user2.ID)
}

func TestUserService_Integration_FindOrCreateFromOAuth_UpdateExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	svc := services.NewUserService(tdb.DB)
	ctx := context.Background()

	// Create user
	info := &oauth.UserInfo{
		Email:     "updateuser@example.com",
		Name:      "Original Name",
		AvatarURL: "",
		ID:        "github-11111",
		Provider:  "github",
	}
	user1, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	// Update with new info
	info.Email = "updated@example.com"
	info.Name = "Updated Name"
	info.AvatarURL = "https://example.com/new-avatar.png"

	user2, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, "updated@example.com", user2.Email)
	assert.Equal(t, "Updated Name", user2.Name)
}

func TestUserService_Integration_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	svc := services.NewUserService(tdb.DB)
	ctx := context.Background()

	// Create user
	info := &oauth.UserInfo{
		Email:    "getbyid@example.com",
		Name:     "Test User",
		ID:       "github-22222",
		Provider: "github",
	}
	created, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	// Get by ID
	user, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, user.ID)
	assert.Equal(t, created.Email, user.Email)
}

func TestUserService_Integration_GetByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	svc := services.NewUserService(tdb.DB)
	ctx := context.Background()

	// Create user
	info := &oauth.UserInfo{
		Email:    "getbyemail@example.com",
		Name:     "Test User",
		ID:       "github-33333",
		Provider: "github",
	}
	created, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	// Get by email
	user, err := svc.GetByEmail(ctx, created.Email)
	require.NoError(t, err)

	assert.Equal(t, created.ID, user.ID)
}

func TestUserService_Integration_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	svc := services.NewUserService(tdb.DB)
	ctx := context.Background()

	// Create user
	info := &oauth.UserInfo{
		Email:    "update@example.com",
		Name:     "Original",
		ID:       "github-44444",
		Provider: "github",
	}
	created, err := svc.FindOrCreateFromOAuth(ctx, info)
	require.NoError(t, err)

	// Update name
	updated, err := svc.Update(ctx, created.ID, "New Name")
	require.NoError(t, err)

	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "New Name", updated.Name)
}
