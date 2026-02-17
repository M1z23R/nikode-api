package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCollectionService(t *testing.T) (*CollectionService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	db := &database.DB{Pool: mock}
	return NewCollectionService(db), mock
}

func TestCollectionService_Create(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()
	name := "Test Collection"
	data := json.RawMessage(`{"key": "value"}`)
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).AddRow(collectionID, workspaceID, name, data, 1, &userID, now, now)

	mock.ExpectQuery(`INSERT INTO collections`).
		WithArgs(workspaceID, name, data, userID).
		WillReturnRows(rows)

	col, err := svc.Create(ctx, workspaceID, name, data, userID)

	require.NoError(t, err)
	assert.Equal(t, collectionID, col.ID)
	assert.Equal(t, name, col.Name)
	assert.Equal(t, 1, col.Version)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Create_NilData(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()
	name := "Test Collection"
	now := time.Now()
	emptyData := json.RawMessage(`{}`)

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).AddRow(collectionID, workspaceID, name, emptyData, 1, &userID, now, now)

	mock.ExpectQuery(`INSERT INTO collections`).
		WithArgs(workspaceID, name, json.RawMessage(`{}`), userID).
		WillReturnRows(rows)

	col, err := svc.Create(ctx, workspaceID, name, nil, userID)

	require.NoError(t, err)
	assert.Equal(t, collectionID, col.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_GetByID(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	now := time.Now()
	data := json.RawMessage(`{}`)

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).AddRow(collectionID, workspaceID, "Test", data, 1, &userID, now, now)

	mock.ExpectQuery(`SELECT .+ FROM collections WHERE id`).
		WithArgs(collectionID).
		WillReturnRows(rows)

	col, err := svc.GetByID(ctx, collectionID)

	require.NoError(t, err)
	assert.Equal(t, collectionID, col.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_GetByID_NotFound(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()

	mock.ExpectQuery(`SELECT .+ FROM collections WHERE id`).
		WithArgs(collectionID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.GetByID(ctx, collectionID)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_GetByWorkspace(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	workspaceID := uuid.New()
	col1ID := uuid.New()
	col2ID := uuid.New()
	userID := uuid.New()
	now := time.Now()
	data := json.RawMessage(`{}`)

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).
		AddRow(col1ID, workspaceID, "Collection 1", data, 1, &userID, now, now).
		AddRow(col2ID, workspaceID, "Collection 2", data, 2, &userID, now, now)

	mock.ExpectQuery(`SELECT .+ FROM collections WHERE workspace_id`).
		WithArgs(workspaceID).
		WillReturnRows(rows)

	collections, err := svc.GetByWorkspace(ctx, workspaceID)

	require.NoError(t, err)
	assert.Len(t, collections, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Update_NameAndData(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	name := "Updated Name"
	data := json.RawMessage(`{"updated": true}`)
	expectedVersion := 1
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).AddRow(collectionID, workspaceID, name, data, 2, &userID, now, now)

	mock.ExpectQuery(`UPDATE collections SET name = .+, data = .+, version = version \+ 1`).
		WithArgs(name, data, userID, collectionID, expectedVersion).
		WillReturnRows(rows)

	col, err := svc.Update(ctx, collectionID, &name, data, expectedVersion, userID)

	require.NoError(t, err)
	assert.Equal(t, name, col.Name)
	assert.Equal(t, 2, col.Version)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Update_NameOnly(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	name := "Updated Name"
	expectedVersion := 1
	now := time.Now()
	data := json.RawMessage(`{}`)

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).AddRow(collectionID, workspaceID, name, data, 2, &userID, now, now)

	mock.ExpectQuery(`UPDATE collections SET name = .+, version = version \+ 1`).
		WithArgs(name, userID, collectionID, expectedVersion).
		WillReturnRows(rows)

	col, err := svc.Update(ctx, collectionID, &name, nil, expectedVersion, userID)

	require.NoError(t, err)
	assert.Equal(t, name, col.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Update_DataOnly(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	data := json.RawMessage(`{"updated": true}`)
	expectedVersion := 1
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "workspace_id", "name", "data", "version", "updated_by", "created_at", "updated_at",
	}).AddRow(collectionID, workspaceID, "Existing Name", data, 2, &userID, now, now)

	mock.ExpectQuery(`UPDATE collections SET data = .+, version = version \+ 1`).
		WithArgs(data, userID, collectionID, expectedVersion).
		WillReturnRows(rows)

	col, err := svc.Update(ctx, collectionID, nil, data, expectedVersion, userID)

	require.NoError(t, err)
	assert.Equal(t, 2, col.Version)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Update_NoFieldsToUpdate(t *testing.T) {
	svc, _ := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	userID := uuid.New()

	_, err := svc.Update(ctx, collectionID, nil, nil, 1, userID)

	assert.ErrorIs(t, err, ErrNoFieldsToUpdate)
}

func TestCollectionService_Update_VersionConflict(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	userID := uuid.New()
	name := "Updated Name"
	expectedVersion := 1
	currentVersion := 2 // Someone else updated it

	// Update returns no rows (version mismatch)
	mock.ExpectQuery(`UPDATE collections SET name = .+, version = version \+ 1`).
		WithArgs(name, userID, collectionID, expectedVersion).
		WillReturnError(pgx.ErrNoRows)

	// checkVersionConflict query
	versionRows := pgxmock.NewRows([]string{"version"}).AddRow(currentVersion)
	mock.ExpectQuery(`SELECT version FROM collections WHERE id`).
		WithArgs(collectionID).
		WillReturnRows(versionRows)

	_, err := svc.Update(ctx, collectionID, &name, nil, expectedVersion, userID)

	assert.ErrorIs(t, err, ErrVersionConflict)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Update_CollectionNotFound(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()
	userID := uuid.New()
	name := "Updated Name"
	expectedVersion := 1

	// Update returns no rows
	mock.ExpectQuery(`UPDATE collections SET name = .+, version = version \+ 1`).
		WithArgs(name, userID, collectionID, expectedVersion).
		WillReturnError(pgx.ErrNoRows)

	// checkVersionConflict - collection doesn't exist
	mock.ExpectQuery(`SELECT version FROM collections WHERE id`).
		WithArgs(collectionID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.Update(ctx, collectionID, &name, nil, expectedVersion, userID)

	assert.ErrorIs(t, err, ErrCollectionNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCollectionService_Delete(t *testing.T) {
	svc, mock := setupCollectionService(t)
	ctx := context.Background()
	collectionID := uuid.New()

	mock.ExpectExec(`DELETE FROM collections WHERE id`).
		WithArgs(collectionID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.Delete(ctx, collectionID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
