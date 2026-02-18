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

func setupWorkspaceService(t *testing.T) (*WorkspaceService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	db := &database.DB{Pool: mock}
	return NewWorkspaceService(db), mock
}

func TestWorkspaceService_Create(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	ownerID := uuid.New()
	workspaceID := uuid.New()
	name := "My Workspace"
	now := time.Now()

	mock.ExpectBegin()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(workspaceID, name, ownerID, now, now)
	mock.ExpectQuery(`INSERT INTO workspaces \(name, owner_id\)`).
		WithArgs(name, ownerID).
		WillReturnRows(rows)

	mock.ExpectExec(`INSERT INTO workspace_members`).
		WithArgs(workspaceID, ownerID, "owner").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectCommit()

	ws, err := svc.Create(ctx, name, ownerID)

	require.NoError(t, err)
	assert.Equal(t, workspaceID, ws.ID)
	assert.Equal(t, name, ws.Name)
	assert.Equal(t, ownerID, ws.OwnerID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_GetByID(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	ownerID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(workspaceID, "Test Workspace", ownerID, now, now)

	mock.ExpectQuery(`SELECT .+ FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	ws, err := svc.GetByID(ctx, workspaceID)

	require.NoError(t, err)
	assert.Equal(t, workspaceID, ws.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_GetByID_NotFound(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
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
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	userID := uuid.New()
	ws1ID := uuid.New()
	ws2ID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at", "role"}).
		AddRow(ws1ID, "Workspace 1", userID, now, now, "owner").
		AddRow(ws2ID, "Workspace 2", uuid.New(), now, now, "member")

	mock.ExpectQuery(`SELECT .+ FROM workspaces w JOIN workspace_members`).
		WithArgs(userID).
		WillReturnRows(rows)

	workspaces, roles, err := svc.GetUserWorkspaces(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, workspaces, 2)
	assert.Len(t, roles, 2)
	assert.Equal(t, "owner", roles[0])
	assert.Equal(t, "member", roles[1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_Update(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	ownerID := uuid.New()
	newName := "Updated Workspace"
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "owner_id", "created_at", "updated_at"}).
		AddRow(workspaceID, newName, ownerID, now, now)

	mock.ExpectQuery(`UPDATE workspaces SET name`).
		WithArgs(newName, workspaceID).
		WillReturnRows(rows)

	ws, err := svc.Update(ctx, workspaceID, newName)

	require.NoError(t, err)
	assert.Equal(t, newName, ws.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_Delete(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()

	mock.ExpectExec(`DELETE FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.Delete(ctx, workspaceID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_IsOwner(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"owner_id"}).AddRow(userID)
	mock.ExpectQuery(`SELECT owner_id FROM workspaces WHERE id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	isOwner, err := svc.IsOwner(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.True(t, isOwner)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceService_IsMember(t *testing.T) {
	svc, mock := setupWorkspaceService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()

	rows := pgxmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(workspaceID, userID).
		WillReturnRows(rows)

	isMember, err := svc.IsMember(ctx, workspaceID, userID)

	require.NoError(t, err)
	assert.True(t, isMember)
	assert.NoError(t, mock.ExpectationsWereMet())
}
