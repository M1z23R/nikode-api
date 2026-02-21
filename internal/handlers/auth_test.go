package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
	driftmw "github.com/m1z23r/drift/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupAuthTest(t *testing.T) (*testutil.MockUserService, *testutil.MockTokenService, *testutil.MockJWTService, *AuthHandler, *config.Config) {
	t.Helper()
	mockUserService := new(testutil.MockUserService)
	mockTokenService := new(testutil.MockTokenService)
	mockJWTService := new(testutil.MockJWTService)

	cfg := &config.Config{
		FrontendCallbackURL: "http://localhost:3000/auth/callback",
	}

	handler := &AuthHandler{
		cfg:          cfg,
		providers:    make(map[string]oauth.Provider),
		userService:  mockUserService,
		tokenService: mockTokenService,
		jwtService:   mockJWTService,
	}

	return mockUserService, mockTokenService, mockJWTService, handler, cfg
}

func TestAuthHandler_ExchangeCode_Success(t *testing.T) {
	mockUserService, mockTokenService, mockJWTService, handler, _ := setupAuthTest(t)

	userID := uuid.New()
	user := &models.User{
		ID:       userID,
		Email:    "test@example.com",
		Name:     "Test User",
		Provider: "github",
	}

	tokenPair := &services.TokenPair{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresIn:    3600,
	}

	// Store an auth code
	authCode := "test-auth-code"
	handler.authCodes.Store(authCode, authCodeData{
		userID:    userID,
		expiresAt: time.Now().Add(30 * time.Second),
	})

	mockUserService.On("GetByID", mock.Anything, userID).Return(user, nil)
	mockJWTService.On("GenerateTokenPair", userID, "test@example.com", mock.Anything).Return(tokenPair, nil)
	mockJWTService.On("RefreshExpiry").Return(7 * 24 * time.Hour)
	mockTokenService.On("StoreRefreshToken", mock.Anything, userID, mock.Anything, mock.Anything).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/exchange", handler.ExchangeCode)

	body := dto.ExchangeCodeRequest{Code: authCode}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.TokenResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "access-token-123", response.AccessToken)
	assert.Equal(t, "refresh-token-456", response.RefreshToken)
	assert.Equal(t, int64(3600), response.ExpiresIn)

	mockUserService.AssertExpectations(t)
	mockJWTService.AssertExpectations(t)
	mockTokenService.AssertExpectations(t)
}

func TestAuthHandler_ExchangeCode_InvalidCode(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/exchange", handler.ExchangeCode)

	body := dto.ExchangeCodeRequest{Code: "invalid-code"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid or expired code")
}

func TestAuthHandler_ExchangeCode_ExpiredCode(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	userID := uuid.New()
	authCode := "expired-auth-code"

	// Store an expired auth code
	handler.authCodes.Store(authCode, authCodeData{
		userID:    userID,
		expiresAt: time.Now().Add(-1 * time.Second), // Already expired
	})

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/exchange", handler.ExchangeCode)

	body := dto.ExchangeCodeRequest{Code: authCode}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "code expired")
}

func TestAuthHandler_ExchangeCode_MissingCode(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/exchange", handler.ExchangeCode)

	body := dto.ExchangeCodeRequest{Code: ""}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "code is required")
}

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	mockUserService, mockTokenService, mockJWTService, handler, _ := setupAuthTest(t)

	userID := uuid.New()
	user := &models.User{
		ID:       userID,
		Email:    "test@example.com",
		Name:     "Test User",
		Provider: "github",
	}

	oldRefreshToken := "old-refresh-token"
	newTokenPair := &services.TokenPair{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresIn:    3600,
	}

	mockJWTService.On("ValidateRefreshToken", oldRefreshToken).Return(userID, nil)
	mockTokenService.On("ValidateRefreshToken", mock.Anything, mock.Anything).Return(userID, nil)
	mockUserService.On("GetByID", mock.Anything, userID).Return(user, nil)
	mockTokenService.On("RevokeRefreshToken", mock.Anything, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", userID, "test@example.com", mock.Anything).Return(newTokenPair, nil)
	mockJWTService.On("RefreshExpiry").Return(7 * 24 * time.Hour)
	mockTokenService.On("StoreRefreshToken", mock.Anything, userID, mock.Anything, mock.Anything).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/refresh", handler.RefreshToken)

	body := dto.RefreshTokenRequest{RefreshToken: oldRefreshToken}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.TokenResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "new-access-token", response.AccessToken)
	assert.Equal(t, "new-refresh-token", response.RefreshToken)

	mockUserService.AssertExpectations(t)
	mockJWTService.AssertExpectations(t)
	mockTokenService.AssertExpectations(t)
}

func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	_, _, mockJWTService, handler, _ := setupAuthTest(t)

	mockJWTService.On("ValidateRefreshToken", "invalid-token").Return(uuid.Nil, errors.New("invalid token"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/refresh", handler.RefreshToken)

	body := dto.RefreshTokenRequest{RefreshToken: "invalid-token"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid refresh token")

	mockJWTService.AssertExpectations(t)
}

func TestAuthHandler_RefreshToken_MissingToken(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/refresh", handler.RefreshToken)

	body := dto.RefreshTokenRequest{RefreshToken: ""}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "refresh_token is required")
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	_, mockTokenService, _, handler, _ := setupAuthTest(t)

	mockTokenService.On("RevokeRefreshToken", mock.Anything, mock.Anything).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/logout", handler.Logout)

	body := dto.RefreshTokenRequest{RefreshToken: "some-refresh-token"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "logged out")

	mockTokenService.AssertExpectations(t)
}

func TestAuthHandler_Logout_EmptyToken(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Post("/auth/logout", handler.Logout)

	body := dto.RefreshTokenRequest{RefreshToken: ""}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	// Should still return success even with empty token
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "logged out")
}

func TestAuthHandler_LogoutAll_Success(t *testing.T) {
	_, mockTokenService, _, handler, _ := setupAuthTest(t)
	jwtSvc := services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)

	userID := uuid.New()
	email := "test@example.com"

	mockTokenService.On("RevokeAllUserTokens", mock.Anything, userID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/auth/logout-all", handler.LogoutAll)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "all sessions logged out")

	mockTokenService.AssertExpectations(t)
}

func TestAuthHandler_LogoutAll_NotAuthenticated(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)
	jwtSvc := services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/auth/logout-all", handler.LogoutAll)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_GetConsentURL_UnsupportedProvider(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/consent", handler.GetConsentURL)

	req := httptest.NewRequest(http.MethodGet, "/auth/unsupported/consent", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "unsupported provider")
}

func TestAuthHandler_GetConsentURL_Success(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	// Add a mock provider
	mockProvider := new(testutil.MockOAuthProvider)
	mockProvider.On("GetConsentURL", mock.AnythingOfType("string")).Return("https://provider.com/auth?state=abc")
	handler.providers["github"] = mockProvider

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/consent", handler.GetConsentURL)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/consent", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.ConsentURLResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.URL, "https://provider.com/auth")

	mockProvider.AssertExpectations(t)
}

// Callback tests

func TestAuthHandler_Callback_UnsupportedProvider(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/unsupported/callback?code=abc&state=xyz", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=unsupported+provider")
}

func TestAuthHandler_Callback_MissingState(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	handler.providers["github"] = mockProvider

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=missing+state+parameter")
}

func TestAuthHandler_Callback_InvalidState(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	handler.providers["github"] = mockProvider

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state=invalid-state", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=invalid+or+expired+state")
}

func TestAuthHandler_Callback_ExpiredState(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	handler.providers["github"] = mockProvider

	// Store an expired state
	state := "expired-state"
	handler.states.Store(state, stateData{expiresAt: time.Now().Add(-1 * time.Minute)})

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state="+state, nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=state+expired")
}

func TestAuthHandler_Callback_MissingCode(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	handler.providers["github"] = mockProvider

	// Store a valid state
	state := "valid-state"
	handler.states.Store(state, stateData{expiresAt: time.Now().Add(10 * time.Minute)})

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state="+state, nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=missing+authorization+code")
}

func TestAuthHandler_Callback_ExchangeCodeError(t *testing.T) {
	_, _, _, handler, _ := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	mockProvider.On("ExchangeCode", mock.Anything, "test-code").Return(nil, errors.New("exchange failed"))
	handler.providers["github"] = mockProvider

	// Store a valid state
	state := "valid-state"
	handler.states.Store(state, stateData{expiresAt: time.Now().Add(10 * time.Minute)})

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=test-code&state="+state, nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=failed+to+exchange+code")

	mockProvider.AssertExpectations(t)
}

func TestAuthHandler_Callback_UserCreationError(t *testing.T) {
	mockUserService, _, _, handler, _ := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	userInfo := &oauth.UserInfo{
		Email:    "test@example.com",
		Name:     "Test User",
		ID:       "12345",
		Provider: "github",
	}
	mockProvider.On("ExchangeCode", mock.Anything, "test-code").Return(userInfo, nil)
	handler.providers["github"] = mockProvider

	mockUserService.On("FindOrCreateFromOAuth", mock.Anything, userInfo).Return(nil, errors.New("db error"))

	// Store a valid state
	state := "valid-state"
	handler.states.Store(state, stateData{expiresAt: time.Now().Add(10 * time.Minute)})

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=test-code&state="+state, nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "error=failed+to+create+user")

	mockProvider.AssertExpectations(t)
	mockUserService.AssertExpectations(t)
}

func TestAuthHandler_Callback_Success(t *testing.T) {
	mockUserService, _, _, handler, cfg := setupAuthTest(t)

	mockProvider := new(testutil.MockOAuthProvider)
	userInfo := &oauth.UserInfo{
		Email:    "test@example.com",
		Name:     "Test User",
		ID:       "12345",
		Provider: "github",
	}
	mockProvider.On("ExchangeCode", mock.Anything, "test-code").Return(userInfo, nil)
	handler.providers["github"] = mockProvider

	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Name:     "Test User",
		Provider: "github",
	}
	mockUserService.On("FindOrCreateFromOAuth", mock.Anything, userInfo).Return(user, nil)

	// Store a valid state
	state := "valid-state"
	handler.states.Store(state, stateData{expiresAt: time.Now().Add(10 * time.Minute)})

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Get("/auth/:provider/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=test-code&state="+state, nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, cfg.FrontendCallbackURL)
	assert.Contains(t, location, "code=")
	assert.NotContains(t, location, "error=")

	mockProvider.AssertExpectations(t)
	mockUserService.AssertExpectations(t)
}
