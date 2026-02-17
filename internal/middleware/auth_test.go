package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJWTService() *services.JWTService {
	return services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)
}

func generateTestToken(t *testing.T, jwtSvc *services.JWTService, userID uuid.UUID, email string) string {
	t.Helper()
	pair, err := jwtSvc.GenerateTokenPair(userID, email)
	require.NoError(t, err)
	return pair.AccessToken
}

func TestAuth_MissingAuthorizationHeader(t *testing.T) {
	jwtSvc := newTestJWTService()
	app := drift.New()

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing authorization header")
}

func TestAuth_InvalidAuthorizationFormat_NoBearer(t *testing.T) {
	jwtSvc := newTestJWTService()
	app := drift.New()

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token some-token")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid authorization header format")
}

func TestAuth_InvalidAuthorizationFormat_OnlyBearer(t *testing.T) {
	jwtSvc := newTestJWTService()
	app := drift.New()

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid authorization header format")
}

func TestAuth_InvalidToken(t *testing.T) {
	jwtSvc := newTestJWTService()
	app := drift.New()

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid or expired token")
}

func TestAuth_ExpiredToken(t *testing.T) {
	// Create service with very short expiry
	jwtSvc := services.NewJWTService("test-secret-key", 1*time.Millisecond, 24*time.Hour)
	app := drift.New()

	userID := uuid.New()
	token := generateTestToken(t, jwtSvc, userID, "test@example.com")

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid or expired token")
}

func TestAuth_WrongSecret(t *testing.T) {
	jwtSvc1 := services.NewJWTService("secret-1", 15*time.Minute, 24*time.Hour)
	jwtSvc2 := services.NewJWTService("secret-2", 15*time.Minute, 24*time.Hour)
	app := drift.New()

	// Generate token with secret-1
	userID := uuid.New()
	token := generateTestToken(t, jwtSvc1, userID, "test@example.com")

	// Validate with secret-2
	app.Use(Auth(jwtSvc2))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_ValidToken(t *testing.T) {
	jwtSvc := newTestJWTService()
	app := drift.New()

	userID := uuid.New()
	email := "test@example.com"
	token := generateTestToken(t, jwtSvc, userID, email)

	var extractedUserID uuid.UUID
	var extractedEmail string

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		extractedUserID = GetUserID(c)
		extractedEmail = GetUserEmail(c)
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, extractedUserID)
	assert.Equal(t, email, extractedEmail)
}

func TestAuth_BearerCaseInsensitive(t *testing.T) {
	jwtSvc := newTestJWTService()
	app := drift.New()

	userID := uuid.New()
	token := generateTestToken(t, jwtSvc, userID, "test@example.com")

	app.Use(Auth(jwtSvc))
	app.Get("/protected", func(c *drift.Context) {
		_ = c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	testCases := []string{"bearer", "BEARER", "BeArEr"}

	for _, bearer := range testCases {
		t.Run(bearer, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", bearer+" "+token)
			rec := httptest.NewRecorder()

			app.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestGetUserID_NotSet(t *testing.T) {
	app := drift.New()

	var extractedUserID uuid.UUID

	app.Get("/test", func(c *drift.Context) {
		extractedUserID = GetUserID(c)
		_ = c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, uuid.Nil, extractedUserID)
}

func TestGetUserEmail_NotSet(t *testing.T) {
	app := drift.New()

	var extractedEmail string

	app.Get("/test", func(c *drift.Context) {
		extractedEmail = GetUserEmail(c)
		_ = c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, "", extractedEmail)
}
