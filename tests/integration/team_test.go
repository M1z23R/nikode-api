package integration

import (
	"context"
	"testing"

	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamService_Integration_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTeamService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)

	team, err := svc.Create(ctx, "Test Team", owner.ID)

	require.NoError(t, err)
	assert.NotEmpty(t, team.ID)
	assert.Equal(t, "Test Team", team.Name)
	assert.Equal(t, owner.ID, team.OwnerID)

	// Verify owner is also a member
	isMember, err := svc.IsMember(ctx, team.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestTeamService_Integration_GetUserTeams(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTeamService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member := fixtures.CreateUser(t)

	// Create team 1 (owner is owner)
	_, err := svc.Create(ctx, "Team 1", owner.ID)
	require.NoError(t, err)

	// Create team 2 (owner is owner, add member)
	team2, err := svc.Create(ctx, "Team 2", owner.ID)
	require.NoError(t, err)
	err = svc.AddMember(ctx, team2.ID, member.ID)
	require.NoError(t, err)

	// Get owner's teams
	ownerTeams, ownerRoles, err := svc.GetUserTeams(ctx, owner.ID)
	require.NoError(t, err)
	assert.Len(t, ownerTeams, 2)
	assert.Equal(t, models.RoleOwner, ownerRoles[0])
	assert.Equal(t, models.RoleOwner, ownerRoles[1])

	// Get member's teams
	memberTeams, memberRoles, err := svc.GetUserTeams(ctx, member.ID)
	require.NoError(t, err)
	assert.Len(t, memberTeams, 1)
	assert.Equal(t, team2.ID, memberTeams[0].ID)
	assert.Equal(t, models.RoleMember, memberRoles[0])
}

func TestTeamService_Integration_AddAndRemoveMember(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTeamService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member := fixtures.CreateUser(t)

	team, err := svc.Create(ctx, "Test Team", owner.ID)
	require.NoError(t, err)

	// Add member
	err = svc.AddMember(ctx, team.ID, member.ID)
	require.NoError(t, err)

	isMember, err := svc.IsMember(ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.True(t, isMember)

	// Remove member
	err = svc.RemoveMember(ctx, team.ID, member.ID)
	require.NoError(t, err)

	isMember, err = svc.IsMember(ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
}

func TestTeamService_Integration_CannotRemoveOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTeamService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)

	team, err := svc.Create(ctx, "Test Team", owner.ID)
	require.NoError(t, err)

	// Try to remove owner
	err = svc.RemoveMember(ctx, team.ID, owner.ID)

	assert.ErrorIs(t, err, services.ErrCannotRemoveOwner)
}

func TestTeamService_Integration_GetMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTeamService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)
	member1 := fixtures.CreateUser(t)
	member2 := fixtures.CreateUser(t)

	team, err := svc.Create(ctx, "Test Team", owner.ID)
	require.NoError(t, err)

	err = svc.AddMember(ctx, team.ID, member1.ID)
	require.NoError(t, err)
	err = svc.AddMember(ctx, team.ID, member2.ID)
	require.NoError(t, err)

	members, err := svc.GetMembers(ctx, team.ID)
	require.NoError(t, err)

	assert.Len(t, members, 3) // owner + 2 members

	// Verify each member has user info populated
	for _, m := range members {
		assert.NotNil(t, m.User)
		assert.NotEmpty(t, m.User.Email)
	}
}

func TestTeamService_Integration_DeleteTeam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTeamService(tdb.DB)
	ctx := context.Background()

	owner := fixtures.CreateUser(t)

	team, err := svc.Create(ctx, "Test Team", owner.ID)
	require.NoError(t, err)

	err = svc.Delete(ctx, team.ID)
	require.NoError(t, err)

	// Should not find team
	_, err = svc.GetByID(ctx, team.ID)
	assert.Error(t, err)
}
