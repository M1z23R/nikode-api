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

func setupTokenService(t *testing.T) (*TokenService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	db := &database.DB{Pool: mock}
	return NewTokenService(db), mock
}

func TestTokenService_StoreRefreshToken(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()
	userID := uuid.New()
	tokenHash := "abc123hash"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(userID, tokenHash, expiresAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := svc.StoreRefreshToken(ctx, userID, tokenHash, expiresAt)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_ValidateRefreshToken_Valid(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()
	userID := uuid.New()
	tokenHash := "valid-hash"

	rows := pgxmock.NewRows([]string{"user_id"}).AddRow(userID)
	mock.ExpectQuery(`SELECT user_id FROM refresh_tokens`).
		WithArgs(tokenHash).
		WillReturnRows(rows)

	result, err := svc.ValidateRefreshToken(ctx, tokenHash)

	assert.NoError(t, err)
	assert.Equal(t, userID, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_ValidateRefreshToken_Expired(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()
	tokenHash := "expired-hash"

	mock.ExpectQuery(`SELECT user_id FROM refresh_tokens`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.ValidateRefreshToken(ctx, tokenHash)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_ValidateRefreshToken_NotFound(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()
	tokenHash := "nonexistent-hash"

	mock.ExpectQuery(`SELECT user_id FROM refresh_tokens`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)

	_, err := svc.ValidateRefreshToken(ctx, tokenHash)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_RevokeRefreshToken(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()
	tokenHash := "to-be-revoked"

	mock.ExpectExec(`DELETE FROM refresh_tokens WHERE token_hash`).
		WithArgs(tokenHash).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := svc.RevokeRefreshToken(ctx, tokenHash)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_RevokeAllUserTokens(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()
	userID := uuid.New()

	mock.ExpectExec(`DELETE FROM refresh_tokens WHERE user_id`).
		WithArgs(userID).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	err := svc.RevokeAllUserTokens(ctx, userID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_CleanupExpired(t *testing.T) {
	svc, mock := setupTokenService(t)
	ctx := context.Background()

	mock.ExpectExec(`DELETE FROM refresh_tokens WHERE expires_at < NOW`).
		WillReturnResult(pgxmock.NewResult("DELETE", 5))

	err := svc.CleanupExpired(ctx)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
