package integration

import (
	"context"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenService_Integration_StoreAndValidate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTokenService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	tokenHash := services.HashToken("my-refresh-token")
	expiresAt := time.Now().Add(24 * time.Hour)

	// Store token
	err := svc.StoreRefreshToken(ctx, user.ID, tokenHash, expiresAt)
	require.NoError(t, err)

	// Validate token
	userID, err := svc.ValidateRefreshToken(ctx, tokenHash)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
}

func TestTokenService_Integration_ValidateExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTokenService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	tokenHash := services.HashToken("expired-token")
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired

	// Store expired token
	err := svc.StoreRefreshToken(ctx, user.ID, tokenHash, expiresAt)
	require.NoError(t, err)

	// Validate should fail
	_, err = svc.ValidateRefreshToken(ctx, tokenHash)
	assert.Error(t, err)
}

func TestTokenService_Integration_RevokeRefreshToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTokenService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	tokenHash := services.HashToken("to-be-revoked")
	expiresAt := time.Now().Add(24 * time.Hour)

	err := svc.StoreRefreshToken(ctx, user.ID, tokenHash, expiresAt)
	require.NoError(t, err)

	// Revoke
	err = svc.RevokeRefreshToken(ctx, tokenHash)
	require.NoError(t, err)

	// Validate should fail
	_, err = svc.ValidateRefreshToken(ctx, tokenHash)
	assert.Error(t, err)
}

func TestTokenService_Integration_RevokeAllUserTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTokenService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Store multiple tokens
	err := svc.StoreRefreshToken(ctx, user.ID, services.HashToken("token-1"), expiresAt)
	require.NoError(t, err)
	err = svc.StoreRefreshToken(ctx, user.ID, services.HashToken("token-2"), expiresAt)
	require.NoError(t, err)
	err = svc.StoreRefreshToken(ctx, user.ID, services.HashToken("token-3"), expiresAt)
	require.NoError(t, err)

	// Revoke all
	err = svc.RevokeAllUserTokens(ctx, user.ID)
	require.NoError(t, err)

	// All should be invalid
	_, err = svc.ValidateRefreshToken(ctx, services.HashToken("token-1"))
	assert.Error(t, err)
	_, err = svc.ValidateRefreshToken(ctx, services.HashToken("token-2"))
	assert.Error(t, err)
	_, err = svc.ValidateRefreshToken(ctx, services.HashToken("token-3"))
	assert.Error(t, err)
}

func TestTokenService_Integration_CleanupExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTest(t)
	fixtures := testutil.NewFixtures(tdb.DB)
	svc := services.NewTokenService(tdb.DB)
	ctx := context.Background()

	user := fixtures.CreateUser(t)

	// Store expired token
	err := svc.StoreRefreshToken(ctx, user.ID, services.HashToken("expired"), time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	// Store valid token
	err = svc.StoreRefreshToken(ctx, user.ID, services.HashToken("valid"), time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	// Cleanup
	err = svc.CleanupExpired(ctx)
	require.NoError(t, err)

	// Valid token should still work
	userID, err := svc.ValidateRefreshToken(ctx, services.HashToken("valid"))
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
}
