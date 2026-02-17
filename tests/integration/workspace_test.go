package integration

import (
	"context"
	"testing"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceService_Integration_CreatePersonal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	teamSvc := services.NewTeamService(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB, teamSvc)
	ctx := context.Background()

	user := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "Personal Workspace", user.ID, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, ws.ID)
	assert.Equal(t, "Personal Workspace", ws.Name)
	assert.NotNil(t, ws.UserID)
	assert.Equal(t, user.ID, *ws.UserID)
	assert.Nil(t, ws.TeamID)
	assert.True(t, ws.IsPersonal())
	assert.False(t, ws.IsTeam())
}

func TestWorkspaceService_Integration_CreateTeam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	teamSvc := services.NewTeamService(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB, teamSvc)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	team := fixtures.CreateTeam(t, user)

	ws, err := svc.Create(ctx, "Team Workspace", user.ID, &team.ID)

	require.NoError(t, err)
	assert.NotNil(t, ws.TeamID)
	assert.Equal(t, team.ID, *ws.TeamID)
	assert.Nil(t, ws.UserID)
	assert.False(t, ws.IsPersonal())
	assert.True(t, ws.IsTeam())
}

func TestWorkspaceService_Integration_GetUserWorkspaces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	teamSvc := services.NewTeamService(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB, teamSvc)
	ctx := context.Background()

	user1 := fixtures.CreateUser(t)
	user2 := fixtures.CreateUser(t)

	// Personal workspace for user1
	_, err := svc.Create(ctx, "Personal", user1.ID, nil)
	require.NoError(t, err)

	// Team workspace (both users are members)
	team := fixtures.CreateTeam(t, user1)
	fixtures.AddTeamMember(t, team, user2)
	_, err = svc.Create(ctx, "Team", user1.ID, &team.ID)
	require.NoError(t, err)

	// Get user1's workspaces (should see both)
	user1Workspaces, err := svc.GetUserWorkspaces(ctx, user1.ID)
	require.NoError(t, err)
	assert.Len(t, user1Workspaces, 2)

	// Get user2's workspaces (should see only team workspace)
	user2Workspaces, err := svc.GetUserWorkspaces(ctx, user2.ID)
	require.NoError(t, err)
	assert.Len(t, user2Workspaces, 1)
	assert.Equal(t, "Team", user2Workspaces[0].Name)
}

func TestWorkspaceService_Integration_CanAccess_PersonalWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	teamSvc := services.NewTeamService(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB, teamSvc)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	other := fixtures.CreateUser(t)

	ws, err := svc.Create(ctx, "Personal", owner.ID, nil)
	require.NoError(t, err)

	// Owner can access
	canAccess, err := svc.CanAccess(ctx, ws.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, canAccess)

	// Other user cannot access
	canAccess, err = svc.CanAccess(ctx, ws.ID, other.ID)
	require.NoError(t, err)
	assert.False(t, canAccess)
}

func TestWorkspaceService_Integration_CanAccess_TeamWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	teamSvc := services.NewTeamService(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB, teamSvc)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member := fixtures.CreateUser(t)
	nonMember := fixtures.CreateUser(t)

	team := fixtures.CreateTeam(t, owner)
	fixtures.AddTeamMember(t, team, member)

	ws, err := svc.Create(ctx, "Team", owner.ID, &team.ID)
	require.NoError(t, err)

	// Team owner can access
	canAccess, err := svc.CanAccess(ctx, ws.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, canAccess)

	// Team member can access
	canAccess, err = svc.CanAccess(ctx, ws.ID, member.ID)
	require.NoError(t, err)
	assert.True(t, canAccess)

	// Non-member cannot access
	canAccess, err = svc.CanAccess(ctx, ws.ID, nonMember.ID)
	require.NoError(t, err)
	assert.False(t, canAccess)
}

func TestWorkspaceService_Integration_CanModify_TeamWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	teamSvc := services.NewTeamService(tdb.DB)
	svc := services.NewWorkspaceService(tdb.DB, teamSvc)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member := fixtures.CreateUser(t)

	team := fixtures.CreateTeam(t, owner)
	fixtures.AddTeamMember(t, team, member)

	ws, err := svc.Create(ctx, "Team", owner.ID, &team.ID)
	require.NoError(t, err)

	// Team owner can modify
	canModify, err := svc.CanModify(ctx, ws.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, canModify)

	// Team member cannot modify (only owner can)
	canModify, err = svc.CanModify(ctx, ws.ID, member.ID)
	require.NoError(t, err)
	assert.False(t, canModify)
}
