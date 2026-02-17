package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTService(t *testing.T) {
	svc := NewJWTService("secret", 15*time.Minute, 24*time.Hour)

	assert.NotNil(t, svc)
	assert.Equal(t, 24*time.Hour, svc.RefreshExpiry())
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()
	email := "test@example.com"

	pair, err := svc.GenerateTokenPair(userID, email)

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, int64(15*60), pair.ExpiresIn) // 15 minutes in seconds
}

func TestJWTService_ValidateAccessToken_Valid(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()
	email := "test@example.com"

	pair, err := svc.GenerateTokenPair(userID, email)
	require.NoError(t, err)

	claims, err := svc.ValidateAccessToken(pair.AccessToken)

	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, "nikode-api", claims.Issuer)
}

func TestJWTService_ValidateAccessToken_WrongSecret(t *testing.T) {
	svc1 := NewJWTService("secret-1", 15*time.Minute, 24*time.Hour)
	svc2 := NewJWTService("secret-2", 15*time.Minute, 24*time.Hour)

	pair, err := svc1.GenerateTokenPair(uuid.New(), "test@example.com")
	require.NoError(t, err)

	_, err = svc2.ValidateAccessToken(pair.AccessToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTService_ValidateAccessToken_Expired(t *testing.T) {
	// Create service with very short expiry
	svc := NewJWTService("test-secret", 1*time.Millisecond, 24*time.Hour)

	pair, err := svc.GenerateTokenPair(uuid.New(), "test@example.com")
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = svc.ValidateAccessToken(pair.AccessToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTService_ValidateAccessToken_MalformedToken(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 24*time.Hour)

	testCases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt-token"},
		{"partial jwt", "eyJhbGciOiJIUzI1NiJ9."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ValidateAccessToken(tc.token)
			assert.Error(t, err)
		})
	}
}

func TestJWTService_ValidateRefreshToken_Valid(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	pair, err := svc.GenerateTokenPair(userID, "test@example.com")
	require.NoError(t, err)

	returnedUserID, err := svc.ValidateRefreshToken(pair.RefreshToken)

	require.NoError(t, err)
	assert.Equal(t, userID, returnedUserID)
}

func TestJWTService_ValidateRefreshToken_WrongSecret(t *testing.T) {
	svc1 := NewJWTService("secret-1", 15*time.Minute, 24*time.Hour)
	svc2 := NewJWTService("secret-2", 15*time.Minute, 24*time.Hour)

	pair, err := svc1.GenerateTokenPair(uuid.New(), "test@example.com")
	require.NoError(t, err)

	_, err = svc2.ValidateRefreshToken(pair.RefreshToken)

	assert.Error(t, err)
}

func TestJWTService_ValidateRefreshToken_Expired(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 1*time.Millisecond)

	pair, err := svc.GenerateTokenPair(uuid.New(), "test@example.com")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = svc.ValidateRefreshToken(pair.RefreshToken)

	assert.Error(t, err)
}

func TestHashToken(t *testing.T) {
	token := "my-refresh-token"

	hash1 := HashToken(token)
	hash2 := HashToken(token)

	// Same token should produce same hash
	assert.Equal(t, hash1, hash2)

	// Hash should be 64 characters (SHA256 hex)
	assert.Len(t, hash1, 64)

	// Different tokens should produce different hashes
	hash3 := HashToken("different-token")
	assert.NotEqual(t, hash1, hash3)
}

func TestJWTService_RefreshTokensAreDifferent(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	pair1, err := svc.GenerateTokenPair(userID, "test@example.com")
	require.NoError(t, err)

	// Wait a bit to ensure different timestamps
	time.Sleep(5 * time.Millisecond)

	pair2, err := svc.GenerateTokenPair(userID, "test@example.com")
	require.NoError(t, err)

	// Refresh tokens should be different due to different JTI (unique ID)
	assert.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken)
}
