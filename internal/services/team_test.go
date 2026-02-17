package services

import (
	"context"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTeamService(t *testing.T) (*TeamService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	db := &database.DB{Pool: mock}
	return NewTeamService(db), mock
}

func TestTeamService_Create(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	ownerID := uuid.New()
	teamID := uuid.New()
	teamName := "Test Team"
	now := time.Now()

	mock.ExpectBegin()

	teamRows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(teamID, teamName, ownerID, now, now)
	mock.ExpectQuery(`INSERT INTO teams`).
		WithArgs(teamName, ownerID).
		WillReturnRows(teamRows)

	mock.ExpectExec(`INSERT INTO team_members`).
		WithArgs(teamID, ownerID, models.RoleOwner).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectCommit()

	team, err := svc.Create(ctx, teamName, ownerID)

	require.NoError(t, err)
	assert.Equal(t, teamID, team.ID)
	assert.Equal(t, teamName, team.Name)
	assert.Equal(t, ownerID, team.OwnerID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_Create_TransactionRollback(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	ownerID := uuid.New()
	teamID := uuid.New()
	teamName := "Test Team"
	now := time.Now()

	mock.ExpectBegin()

	teamRows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(teamID, teamName, ownerID, now, now)
	mock.ExpectQuery(`INSERT INTO teams`).
		WithArgs(teamName, ownerID).
		WillReturnRows(teamRows)

	// Member insert fails
	mock.ExpectExec(`INSERT INTO team_members`).
		WithArgs(teamID, ownerID, models.RoleOwner).
		WillReturnError(assert.AnError)

	mock.ExpectRollback()

	_, err := svc.Create(ctx, teamName, ownerID)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_GetByID(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	ownerID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(teamID, "Test Team", ownerID, now, now)

	mock.ExpectQuery(`SELECT .+ FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnRows(rows)

	team, err := svc.GetByID(ctx, teamID)

	require.NoError(t, err)
	assert.Equal(t, teamID, team.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_GetByID_NotFound(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	mock.ExpectQuery(`SELECT .+ FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.GetByID(ctx, teamID)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_GetUserTeams(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	userID := uuid.New()
	teamID1 := uuid.New()
	teamID2 := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at", "role"}).
		AddRow(teamID1, "Team 1", userID, now, now, models.RoleOwner).
		AddRow(teamID2, "Team 2", uuid.New(), now, now, models.RoleMember)

	mock.ExpectQuery(`SELECT .+ FROM teams t JOIN team_members tm`).
		WithArgs(userID).
		WillReturnRows(rows)

	teams, roles, err := svc.GetUserTeams(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, teams, 2)
	assert.Len(t, roles, 2)
	assert.Equal(t, models.RoleOwner, roles[0])
	assert.Equal(t, models.RoleMember, roles[1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_Update(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	ownerID := uuid.New()
	newName := "Updated Team"
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(teamID, newName, ownerID, now, now)

	mock.ExpectQuery(`UPDATE teams SET name`).
		WithArgs(newName, teamID).
		WillReturnRows(rows)

	team, err := svc.Update(ctx, teamID, newName)

	require.NoError(t, err)
	assert.Equal(t, newName, team.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_Delete(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	mock.ExpectExec(`DELETE FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.Delete(ctx, teamID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_IsOwner_True(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"owner_id"}).AddRow(userID)
	mock.ExpectQuery(`SELECT owner_id FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnRows(rows)

	isOwner, err := svc.IsOwner(ctx, teamID, userID)

	require.NoError(t, err)
	assert.True(t, isOwner)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_IsOwner_False(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()

	rows := pgxmock.NewRows([]string{"owner_id"}).AddRow(otherUserID)
	mock.ExpectQuery(`SELECT owner_id FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnRows(rows)

	isOwner, err := svc.IsOwner(ctx, teamID, userID)

	require.NoError(t, err)
	assert.False(t, isOwner)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_IsMember_True(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(teamID, userID).
		WillReturnRows(rows)

	isMember, err := svc.IsMember(ctx, teamID, userID)

	require.NoError(t, err)
	assert.True(t, isMember)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_IsMember_False(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"exists"}).AddRow(false)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(teamID, userID).
		WillReturnRows(rows)

	isMember, err := svc.IsMember(ctx, teamID, userID)

	require.NoError(t, err)
	assert.False(t, isMember)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_GetMembers(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	memberID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"tm_id", "tm_team_id", "tm_user_id", "tm_role", "tm_created_at",
		"u_id", "u_email", "u_name", "u_avatar_url", "u_provider", "u_created_at", "u_updated_at",
	}).AddRow(
		memberID, teamID, userID, models.RoleMember, now,
		userID, "user@example.com", "Test User", nil, "github", now, now,
	)

	mock.ExpectQuery(`SELECT .+ FROM team_members tm JOIN users u`).
		WithArgs(teamID).
		WillReturnRows(rows)

	members, err := svc.GetMembers(ctx, teamID)

	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, models.RoleMember, members[0].Role)
	assert.NotNil(t, members[0].User)
	assert.Equal(t, "user@example.com", members[0].User.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_AddMember(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	mock.ExpectExec(`INSERT INTO team_members .+ ON CONFLICT .+ DO NOTHING`).
		WithArgs(teamID, userID, models.RoleMember).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := svc.AddMember(ctx, teamID, userID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_RemoveMember_Success(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"role"}).AddRow(models.RoleMember)
	mock.ExpectQuery(`SELECT role FROM team_members WHERE team_id`).
		WithArgs(teamID, userID).
		WillReturnRows(rows)

	mock.ExpectExec(`DELETE FROM team_members WHERE team_id`).
		WithArgs(teamID, userID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.RemoveMember(ctx, teamID, userID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_RemoveMember_CannotRemoveOwner(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"role"}).AddRow(models.RoleOwner)
	mock.ExpectQuery(`SELECT role FROM team_members WHERE team_id`).
		WithArgs(teamID, userID).
		WillReturnRows(rows)

	err := svc.RemoveMember(ctx, teamID, userID)

	assert.ErrorIs(t, err, ErrCannotRemoveOwner)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_RemoveMember_NotFound(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	userID := uuid.New()

	mock.ExpectQuery(`SELECT role FROM team_members WHERE team_id`).
		WithArgs(teamID, userID).
		WillReturnError(pgx.ErrNoRows)

	err := svc.RemoveMember(ctx, teamID, userID)

	assert.ErrorIs(t, err, ErrMemberNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// CreateInvite tests

func TestTeamService_CreateInvite_Success(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	inviteID := uuid.New()
	now := time.Now()

	// Check if user is already a member
	memberRows := pgxmock.NewRows([]string{"exists"}).AddRow(false)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(teamID, inviteeID).
		WillReturnRows(memberRows)

	// Insert invite
	inviteRows := pgxmock.NewRows([]string{
		"id", "team_id", "inviter_id", "invitee_id", "status", "created_at", "updated_at",
	}).AddRow(inviteID, teamID, inviterID, inviteeID, models.InviteStatusPending, now, now)

	mock.ExpectQuery(`INSERT INTO team_invites`).
		WithArgs(teamID, inviterID, inviteeID, models.InviteStatusPending).
		WillReturnRows(inviteRows)

	invite, err := svc.CreateInvite(ctx, teamID, inviterID, inviteeID)

	require.NoError(t, err)
	assert.Equal(t, inviteID, invite.ID)
	assert.Equal(t, teamID, invite.TeamID)
	assert.Equal(t, models.InviteStatusPending, invite.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_CreateInvite_AlreadyMember(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()

	// User is already a member
	memberRows := pgxmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(teamID, inviteeID).
		WillReturnRows(memberRows)

	_, err := svc.CreateInvite(ctx, teamID, inviterID, inviteeID)

	assert.ErrorIs(t, err, ErrAlreadyMember)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// GetInviteByID tests

func TestTeamService_GetInviteByID_Success(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "team_id", "inviter_id", "invitee_id", "status", "created_at", "updated_at",
	}).AddRow(inviteID, teamID, inviterID, inviteeID, models.InviteStatusPending, now, now)

	mock.ExpectQuery(`SELECT .+ FROM team_invites WHERE id`).
		WithArgs(inviteID).
		WillReturnRows(rows)

	invite, err := svc.GetInviteByID(ctx, inviteID)

	require.NoError(t, err)
	assert.Equal(t, inviteID, invite.ID)
	assert.Equal(t, teamID, invite.TeamID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_GetInviteByID_NotFound(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()

	mock.ExpectQuery(`SELECT .+ FROM team_invites WHERE id`).
		WithArgs(inviteID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.GetInviteByID(ctx, inviteID)

	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// GetUserPendingInvites tests

func TestTeamService_GetUserPendingInvites(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	userID := uuid.New()
	inviteID := uuid.New()
	teamID := uuid.New()
	inviterID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"ti_id", "ti_team_id", "ti_inviter_id", "ti_invitee_id", "ti_status", "ti_created_at", "ti_updated_at",
		"t_id", "t_name", "t_owner_id", "t_created_at", "t_updated_at",
		"u_id", "u_email", "u_name", "u_avatar_url", "u_provider", "u_created_at", "u_updated_at",
	}).AddRow(
		inviteID, teamID, inviterID, userID, models.InviteStatusPending, now, now,
		teamID, "Test Team", inviterID, now, now,
		inviterID, "inviter@example.com", "Inviter", nil, "github", now, now,
	)

	mock.ExpectQuery(`SELECT .+ FROM team_invites ti JOIN teams t ON ti.team_id = t.id JOIN users u ON ti.inviter_id = u.id`).
		WithArgs(userID, models.InviteStatusPending).
		WillReturnRows(rows)

	invites, err := svc.GetUserPendingInvites(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, invites, 1)
	assert.Equal(t, inviteID, invites[0].ID)
	assert.NotNil(t, invites[0].Team)
	assert.NotNil(t, invites[0].Inviter)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// GetTeamPendingInvites tests

func TestTeamService_GetTeamPendingInvites(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	inviteID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"ti_id", "ti_team_id", "ti_inviter_id", "ti_invitee_id", "ti_status", "ti_created_at", "ti_updated_at",
		"u_id", "u_email", "u_name", "u_avatar_url", "u_provider", "u_created_at", "u_updated_at",
	}).AddRow(
		inviteID, teamID, inviterID, inviteeID, models.InviteStatusPending, now, now,
		inviteeID, "invitee@example.com", "Invitee", nil, "github", now, now,
	)

	mock.ExpectQuery(`SELECT .+ FROM team_invites ti JOIN users u ON ti.invitee_id = u.id`).
		WithArgs(teamID, models.InviteStatusPending).
		WillReturnRows(rows)

	invites, err := svc.GetTeamPendingInvites(ctx, teamID)

	require.NoError(t, err)
	assert.Len(t, invites, 1)
	assert.Equal(t, inviteID, invites[0].ID)
	assert.NotNil(t, invites[0].Invitee)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// AcceptInvite tests

func TestTeamService_AcceptInvite_Success(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	teamID := uuid.New()
	userID := uuid.New()

	mock.ExpectBegin()

	// Get invite
	inviteRows := pgxmock.NewRows([]string{"id", "team_id", "invitee_id", "status"}).
		AddRow(inviteID, teamID, userID, models.InviteStatusPending)
	mock.ExpectQuery(`SELECT .+ FROM team_invites WHERE id`).
		WithArgs(inviteID).
		WillReturnRows(inviteRows)

	// Update invite status
	mock.ExpectExec(`UPDATE team_invites SET status`).
		WithArgs(models.InviteStatusAccepted, inviteID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Add member
	mock.ExpectExec(`INSERT INTO team_members`).
		WithArgs(teamID, userID, models.RoleMember).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectCommit()

	err := svc.AcceptInvite(ctx, inviteID, userID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_AcceptInvite_NotFound(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	userID := uuid.New()

	mock.ExpectBegin()

	mock.ExpectQuery(`SELECT .+ FROM team_invites WHERE id`).
		WithArgs(inviteID).
		WillReturnError(pgx.ErrNoRows)

	mock.ExpectRollback()

	err := svc.AcceptInvite(ctx, inviteID, userID)

	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_AcceptInvite_WrongUser(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	teamID := uuid.New()
	inviteeID := uuid.New()
	wrongUserID := uuid.New()

	mock.ExpectBegin()

	inviteRows := pgxmock.NewRows([]string{"id", "team_id", "invitee_id", "status"}).
		AddRow(inviteID, teamID, inviteeID, models.InviteStatusPending)
	mock.ExpectQuery(`SELECT .+ FROM team_invites WHERE id`).
		WithArgs(inviteID).
		WillReturnRows(inviteRows)

	mock.ExpectRollback()

	err := svc.AcceptInvite(ctx, inviteID, wrongUserID)

	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_AcceptInvite_AlreadyProcessed(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	teamID := uuid.New()
	userID := uuid.New()

	mock.ExpectBegin()

	inviteRows := pgxmock.NewRows([]string{"id", "team_id", "invitee_id", "status"}).
		AddRow(inviteID, teamID, userID, models.InviteStatusAccepted)
	mock.ExpectQuery(`SELECT .+ FROM team_invites WHERE id`).
		WithArgs(inviteID).
		WillReturnRows(inviteRows)

	mock.ExpectRollback()

	err := svc.AcceptInvite(ctx, inviteID, userID)

	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DeclineInvite tests

func TestTeamService_DeclineInvite_Success(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	userID := uuid.New()

	mock.ExpectExec(`UPDATE team_invites SET status`).
		WithArgs(models.InviteStatusDeclined, inviteID, userID, models.InviteStatusPending).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := svc.DeclineInvite(ctx, inviteID, userID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_DeclineInvite_NotFound(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	userID := uuid.New()

	mock.ExpectExec(`UPDATE team_invites SET status`).
		WithArgs(models.InviteStatusDeclined, inviteID, userID, models.InviteStatusPending).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := svc.DeclineInvite(ctx, inviteID, userID)

	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// CancelInvite tests

func TestTeamService_CancelInvite_Success(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	teamID := uuid.New()

	mock.ExpectExec(`DELETE FROM team_invites WHERE id`).
		WithArgs(inviteID, teamID, models.InviteStatusPending).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.CancelInvite(ctx, inviteID, teamID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamService_CancelInvite_NotFound(t *testing.T) {
	svc, mock := setupTeamService(t)
	ctx := context.Background()
	inviteID := uuid.New()
	teamID := uuid.New()

	mock.ExpectExec(`DELETE FROM team_invites WHERE id`).
		WithArgs(inviteID, teamID, models.InviteStatusPending).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err := svc.CancelInvite(ctx, inviteID, teamID)

	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}
