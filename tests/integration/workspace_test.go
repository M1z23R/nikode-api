package integration

import (
	"context"
	"testing"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceService_Integration_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "My Workspace", user.ID)

	require.NoError(t, err)
	assert.NotEmpty(t, ws.ID)
	assert.Equal(t, "My Workspace", ws.Name)
	assert.Equal(t, user.ID, ws.OwnerID)
}

func TestWorkspaceService_Integration_GetUserWorkspaces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB)
	ctx := context.Background()

	user1 := fixtures.CreateUser(t)
	user2 := fixtures.CreateUser(t)

	// Create workspace owned by user1
	ws, err := svc.Create(ctx, "User1 Workspace", user1.ID)
	require.NoError(t, err)

	// Add user2 as member
	fixtures.AddWorkspaceMember(t, ws, user2)

	// Get user1's workspaces (should see workspace as owner)
	user1Workspaces, user1Roles, err := svc.GetUserWorkspaces(ctx, user1.ID)
	require.NoError(t, err)
	assert.Len(t, user1Workspaces, 1)
	assert.Equal(t, "owner", user1Roles[0])

	// Get user2's workspaces (should see workspace as member)
	user2Workspaces, user2Roles, err := svc.GetUserWorkspaces(ctx, user2.ID)
	require.NoError(t, err)
	assert.Len(t, user2Workspaces, 1)
	assert.Equal(t, "member", user2Roles[0])
}

func TestWorkspaceService_Integration_CanAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member := fixtures.CreateUser(t)
	nonMember := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "Test Workspace", owner.ID)
	require.NoError(t, err)

	// Add member
	fixtures.AddWorkspaceMember(t, ws, member)

	// Owner can access
	canAccess, err := svc.CanAccess(ctx, ws.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, canAccess)

	// Member can access
	canAccess, err = svc.CanAccess(ctx, ws.ID, member.ID)
	require.NoError(t, err)
	assert.True(t, canAccess)

	// Non-member cannot access
	canAccess, err = svc.CanAccess(ctx, ws.ID, nonMember.ID)
	require.NoError(t, err)
	assert.False(t, canAccess)
}

func TestWorkspaceService_Integration_CanModify(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "Test Workspace", owner.ID)
	require.NoError(t, err)

	// Add member
	fixtures.AddWorkspaceMember(t, ws, member)

	// Owner can modify
	canModify, err := svc.CanModify(ctx, ws.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, canModify)

	// Member cannot modify (only owner can)
	canModify, err = svc.CanModify(ctx, ws.ID, member.ID)
	require.NoError(t, err)
	assert.False(t, canModify)
}

func TestWorkspaceService_Integration_AddAndRemoveMember(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	newMember := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "Test Workspace", owner.ID)
	require.NoError(t, err)

	// Initially, new member cannot access
	canAccess, err := svc.CanAccess(ctx, ws.ID, newMember.ID)
	require.NoError(t, err)
	assert.False(t, canAccess)

	// Add member
	err = svc.AddMember(ctx, ws.ID, newMember.ID)
	require.NoError(t, err)

	// Now member can access
	canAccess, err = svc.CanAccess(ctx, ws.ID, newMember.ID)
	require.NoError(t, err)
	assert.True(t, canAccess)

	// Remove member
	err = svc.RemoveMember(ctx, ws.ID, newMember.ID)
	require.NoError(t, err)

	// Member can no longer access
	canAccess, err = svc.CanAccess(ctx, ws.ID, newMember.ID)
	require.NoError(t, err)
	assert.False(t, canAccess)
}

func TestWorkspaceService_Integration_GetMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member1 := fixtures.CreateUser(t)
	member2 := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "Test Workspace", owner.ID)
	require.NoError(t, err)

	// Add members
	err = svc.AddMember(ctx, ws.ID, member1.ID)
	require.NoError(t, err)
	err = svc.AddMember(ctx, ws.ID, member2.ID)
	require.NoError(t, err)

	// Get members
	members, err := svc.GetMembers(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, members, 3) // owner + 2 members

	// Check that owner is included
	hasOwner := false
	for _, m := range members {
		if m.UserID == owner.ID && m.Role == "owner" {
			hasOwner = true
			break
		}
	}
	assert.True(t, hasOwner)
}
