package services

import (
	"context"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWorkspaceService(t *testing.T) (*WorkspaceService, *TeamService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	db := &database.DB{Pool: mock}
	teamSvc := NewTeamService(db)
	return NewWorkspaceService(db, teamSvc), teamSvc, mock
}

func TestWorkspaceService_Create_Personal(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	userID := uuid.New()
	workspaceID := uuid.New()
	name := "My Workspace"
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, name, &userID, nil, now, now)

	mock.ExpectQuery(`INSERT INTO workspaces \(name, user_id\)`).
		WithArgs(name, userID).
		WillReturnRows(rows)

	ws, err := svc.Create(ctx, name, userID, nil)

	require.NoError(t, err)
	assert.Equal(t, workspaceID, ws.ID)
	assert.Equal(t, name, ws.Name)
	assert.NotNil(t, ws.UserID)
	assert.Equal(t, userID, *ws.UserID)
	assert.Nil(t, ws.TeamID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_Create_Team(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	workspaceID := uuid.New()
	name := "Team Workspace"
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, name, nil, &teamID, now, now)

	mock.ExpectQuery(`INSERT INTO workspaces \(name, team_id\)`).
		WithArgs(name, &teamID).
		WillReturnRows(rows)

	ws, err := svc.Create(ctx, name, userID, &teamID)

	require.NoError(t, err)
	assert.Equal(t, workspaceID, ws.ID)
	assert.Nil(t, ws.UserID)
	assert.NotNil(t, ws.TeamID)
	assert.Equal(t, teamID, *ws.TeamID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_GetByID(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test Workspace", &userID, nil, now, now)

	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	ws, err := svc.GetByID(ctx, workspaceID)

	require.NoError(t, err)
	assert.Equal(t, workspaceID, ws.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_GetByID_NotFound(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()

	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.GetByID(ctx, workspaceID)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_GetUserWorkspaces(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	userID := uuid.New()
	ws1ID := uuid.New()
	ws2ID := uuid.New()
	teamID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(ws1ID, "Personal", &userID, nil, now, now).
		AddRow(ws2ID, "Team", nil, &teamID, now, now)

	mock.ExpectQuery(`SELECT DISTINCT .+ FROM workspaces w LEFT JOIN team_members`).
		WithArgs(userID).
		WillReturnRows(rows)

	workspaces, err := svc.GetUserWorkspaces(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, workspaces, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_Update(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()
	newName := "Updated Workspace"
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, newName, &userID, nil, now, now)

	mock.ExpectQuery(`UPDATE workspaces SET name`).
		WithArgs(newName, workspaceID).
		WillReturnRows(rows)

	ws, err := svc.Update(ctx, workspaceID, newName)

	require.NoError(t, err)
	assert.Equal(t, newName, ws.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_Delete(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()

	mock.ExpectExec(`DELETE FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.Delete(ctx, workspaceID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanAccess_PersonalOwner(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", &userID, nil, now, now)

	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	canAccess, err := svc.CanAccess(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.True(t, canAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanAccess_PersonalNotOwner(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", &ownerID, nil, now, now)

	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	canAccess, err := svc.CanAccess(ctx, workspaceID, otherUserID)

	require.NoError(t, err)
	assert.False(t, canAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanAccess_TeamMember(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	teamID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	// GetByID
	wsRows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", nil, &teamID, now, now)
	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(wsRows)

	// IsMember check
	memberRows := pgxmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(teamID, userID).
		WillReturnRows(memberRows)

	canAccess, err := svc.CanAccess(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.True(t, canAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanAccess_TeamNonMember(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	teamID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	// GetByID
	wsRows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", nil, &teamID, now, now)
	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(wsRows)

	// IsMember check
	memberRows := pgxmock.NewRows([]string{"exists"}).AddRow(false)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(teamID, userID).
		WillReturnRows(memberRows)

	canAccess, err := svc.CanAccess(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.False(t, canAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanModify_PersonalOwner(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", &userID, nil, now, now)

	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	canModify, err := svc.CanModify(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.True(t, canModify)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanModify_TeamOwner(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	teamID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	// GetByID
	wsRows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", nil, &teamID, now, now)
	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(wsRows)

	// IsOwner check
	ownerRows := pgxmock.NewRows([]string{"owner_id"}).AddRow(userID)
	mock.ExpectQuery(`SELECT owner_id FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnRows(ownerRows)

	canModify, err := svc.CanModify(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.True(t, canModify)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_CanModify_TeamMemberNotOwner(t *testing.T) {
	svc, _, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	teamID := uuid.New()
	userID := uuid.New()
	teamOwnerID := uuid.New()
	now := time.Now()

	// GetByID
	wsRows := pgxmock.NewRows([]string{"id", "name", "user_id", "team_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test", nil, &teamID, now, now)
	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(wsRows)

	// IsOwner check - user is not the team owner
	ownerRows := pgxmock.NewRows([]string{"owner_id"}).AddRow(teamOwnerID)
	mock.ExpectQuery(`SELECT owner_id FROM teams WHERE id`).
		WithArgs(teamID).
		WillReturnRows(ownerRows)

	canModify, err := svc.CanModify(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.False(t, canModify)
	assert.NoError(t, mock.ExpectationsWereMet())
}
