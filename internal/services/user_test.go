package services

import (
	"context"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserService(t *testing.T) (*UserService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	db := &database.DB{Pool: mock}
	return NewUserService(db), mock
}

func TestUserService_FindOrCreateFromOAuth_CreateNew(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	info := &oauth.UserInfo{
		Email:     "new@example.com",
		Name:      "New User",
		AvatarURL: "https://example.com/avatar.png",
		ID:        "provider-123",
		Provider:  "github",
	}
	userID := uuid.New()
	now := time.Now()

	// First query - user not found
	mock.ExpectQuery(`SELECT .+ FROM users WHERE provider = .+ AND provider_id`).
		WithArgs(info.Provider, info.ID).
		WillReturnError(pgx.ErrNoRows)

	// Insert new user
	rows := pgxmock.NewRows([]string{
		"id", "email", "name", "avatar_url", "provider", "provider_id", "global_role", "created_at", "updated_at",
	}).AddRow(userID, info.Email, info.Name, &info.AvatarURL, info.Provider, info.ID, "user", now, now)

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(info.Email, info.Name, &info.AvatarURL, info.Provider, info.ID).
		WillReturnRows(rows)

	user, err := svc.FindOrCreateFromOAuth(ctx, info)

	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, info.Email, user.Email)
	assert.Equal(t, info.Name, user.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_FindOrCreateFromOAuth_FindExisting(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	info := &oauth.UserInfo{
		Email:     "existing@example.com",
		Name:      "Existing User",
		AvatarURL: "https://example.com/avatar.png",
		ID:        "provider-456",
		Provider:  "github",
	}
	userID := uuid.New()
	now := time.Now()
	avatarURL := "https://example.com/avatar.png"

	// User found
	rows := pgxmock.NewRows([]string{
		"id", "email", "name", "avatar_url", "provider", "provider_id", "global_role", "created_at", "updated_at",
	}).AddRow(userID, info.Email, info.Name, &avatarURL, info.Provider, info.ID, "user", now, now)

	mock.ExpectQuery(`SELECT .+ FROM users WHERE provider = .+ AND provider_id`).
		WithArgs(info.Provider, info.ID).
		WillReturnRows(rows)

	user, err := svc.FindOrCreateFromOAuth(ctx, info)

	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, info.Email, user.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_FindOrCreateFromOAuth_UpdateExisting(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	info := &oauth.UserInfo{
		Email:     "updated@example.com",
		Name:      "Updated Name",
		AvatarURL: "https://example.com/new-avatar.png",
		ID:        "provider-789",
		Provider:  "github",
	}
	userID := uuid.New()
	now := time.Now()

	// User found with different email/name
	rows := pgxmock.NewRows([]string{
		"id", "email", "name", "avatar_url", "provider", "provider_id", "global_role", "created_at", "updated_at",
	}).AddRow(userID, "old@example.com", "Old Name", nil, info.Provider, info.ID, "user", now, now)

	mock.ExpectQuery(`SELECT .+ FROM users WHERE provider = .+ AND provider_id`).
		WithArgs(info.Provider, info.ID).
		WillReturnRows(rows)

	// Update triggered
	mock.ExpectExec(`UPDATE users SET email = .+, name = .+, avatar_url`).
		WithArgs(info.Email, info.Name, &info.AvatarURL, userID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	user, err := svc.FindOrCreateFromOAuth(ctx, info)

	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, info.Email, user.Email)
	assert.Equal(t, info.Name, user.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetByID(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	userID := uuid.New()
	now := time.Now()
	avatarURL := "https://example.com/avatar.png"

	rows := pgxmock.NewRows([]string{
		"id", "email", "name", "avatar_url", "provider", "provider_id", "global_role", "created_at", "updated_at",
	}).AddRow(userID, "test@example.com", "Test User", &avatarURL, "github", "123", "user", now, now)

	mock.ExpectQuery(`SELECT .+ FROM users WHERE id`).
		WithArgs(userID).
		WillReturnRows(rows)

	user, err := svc.GetByID(ctx, userID)

	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	userID := uuid.New()

	mock.ExpectQuery(`SELECT .+ FROM users WHERE id`).
		WithArgs(userID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.GetByID(ctx, userID)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetByEmail(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	userID := uuid.New()
	email := "find@example.com"
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "email", "name", "avatar_url", "provider", "provider_id", "global_role", "created_at", "updated_at",
	}).AddRow(userID, email, "Test User", nil, "github", "123", "user", now, now)

	mock.ExpectQuery(`SELECT .+ FROM users WHERE email`).
		WithArgs(email).
		WillReturnRows(rows)

	user, err := svc.GetByEmail(ctx, email)

	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, email, user.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetByEmail_NotFound(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	email := "notfound@example.com"

	mock.ExpectQuery(`SELECT .+ FROM users WHERE email`).
		WithArgs(email).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.GetByEmail(ctx, email)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_Update(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	userID := uuid.New()
	newName := "Updated Name"
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "email", "name", "avatar_url", "provider", "provider_id", "global_role", "created_at", "updated_at",
	}).AddRow(userID, "test@example.com", newName, nil, "github", "123", "user", now, now)

	mock.ExpectQuery(`UPDATE users SET name = .+ WHERE id`).
		WithArgs(newName, userID).
		WillReturnRows(rows)

	user, err := svc.Update(ctx, userID, newName)

	require.NoError(t, err)
	assert.Equal(t, newName, user.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_Update_NotFound(t *testing.T) {
	svc, mock := setupUserService(t)
	ctx := context.Background()
	userID := uuid.New()
	newName := "Updated Name"

	mock.ExpectQuery(`UPDATE users SET name = .+ WHERE id`).
		WithArgs(newName, userID).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.Update(ctx, userID, newName)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
