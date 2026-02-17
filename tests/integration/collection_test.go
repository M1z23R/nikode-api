package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionService_Integration_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	data := json.RawMessage(`{"items": []}`)
	col, err := svc.Create(ctx, ws.ID, "Test Collection", data, user.ID)

	require.NoError(t, err)
	assert.NotEmpty(t, col.ID)
	assert.Equal(t, "Test Collection", col.Name)
	assert.Equal(t, ws.ID, col.WorkspaceID)
	assert.Equal(t, 1, col.Version)
	assert.NotNil(t, col.UpdatedBy)
	assert.Equal(t, user.ID, *col.UpdatedBy)
}

func TestCollectionService_Integration_GetByWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	// Create multiple collections
	_, err := svc.Create(ctx, ws.ID, "Collection 1", nil, user.ID)
	require.NoError(t, err)
	_, err = svc.Create(ctx, ws.ID, "Collection 2", nil, user.ID)
	require.NoError(t, err)

	collections, err := svc.GetByWorkspace(ctx, ws.ID)

	require.NoError(t, err)
	assert.Len(t, collections, 2)
}

func TestCollectionService_Integration_Update_OptimisticLocking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	// Create collection (version 1)
	col, err := svc.Create(ctx, ws.ID, "Test Collection", nil, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, col.Version)

	// Update with correct version (1 -> 2)
	newName := "Updated Name"
	updated, err := svc.Update(ctx, col.ID, &newName, nil, 1, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	assert.Equal(t, "Updated Name", updated.Name)

	// Update again with correct version (2 -> 3)
	anotherName := "Another Update"
	updated, err = svc.Update(ctx, col.ID, &anotherName, nil, 2, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, updated.Version)
}

func TestCollectionService_Integration_Update_VersionConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	// Create collection (version 1)
	col, err := svc.Create(ctx, ws.ID, "Test Collection", nil, user.ID)
	require.NoError(t, err)

	// Update to version 2
	name1 := "First Update"
	_, err = svc.Update(ctx, col.ID, &name1, nil, 1, user.ID)
	require.NoError(t, err)

	// Try to update with stale version (1, but current is 2)
	name2 := "Stale Update"
	_, err = svc.Update(ctx, col.ID, &name2, nil, 1, user.ID)

	assert.ErrorIs(t, err, services.ErrVersionConflict)
}

func TestCollectionService_Integration_Update_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	// Create and delete collection
	col, err := svc.Create(ctx, ws.ID, "Test Collection", nil, user.ID)
	require.NoError(t, err)

	err = svc.Delete(ctx, col.ID)
	require.NoError(t, err)

	// Try to update deleted collection
	name := "Update"
	_, err = svc.Update(ctx, col.ID, &name, nil, 1, user.ID)

	assert.ErrorIs(t, err, services.ErrCollectionNotFound)
}

func TestCollectionService_Integration_Update_DataOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	col, err := svc.Create(ctx, ws.ID, "Test Collection", json.RawMessage(`{}`), user.ID)
	require.NoError(t, err)

	// Update data only
	newData := json.RawMessage(`{"key": "value", "nested": {"a": 1}}`)
	updated, err := svc.Update(ctx, col.ID, nil, newData, 1, user.ID)

	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	assert.Equal(t, "Test Collection", updated.Name) // Name unchanged
	assert.JSONEq(t, `{"key": "value", "nested": {"a": 1}}`, string(updated.Data))
}

func TestCollectionService_Integration_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewCollectionService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	ws := fixtures.CreateWorkspace(t, testutil.WithUser(user))

	col, err := svc.Create(ctx, ws.ID, "Test Collection", nil, user.ID)
	require.NoError(t, err)

	err = svc.Delete(ctx, col.ID)
	require.NoError(t, err)

	// Should not find collection
	_, err = svc.GetByID(ctx, col.ID)
	assert.Error(t, err)
}
