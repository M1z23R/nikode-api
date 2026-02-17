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
