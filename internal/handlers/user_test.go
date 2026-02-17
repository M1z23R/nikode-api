package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/models"
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

func newTestJWTService() *services.JWTService {
	return services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)
}

func generateTestToken(t *testing.T, jwtSvc *services.JWTService, userID uuid.UUID, email string) string {
	t.Helper()
	pair, err := jwtSvc.GenerateTokenPair(userID, email)
	require.NoError(t, err)
	return pair.AccessToken
}

func TestUserHandler_GetMe_Success(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	userID := uuid.New()
	email := "test@example.com"
	avatarURL := "https://example.com/avatar.png"
	user := &models.User{
		ID:        userID,
		Email:     email,
		Name:      "Test User",
		AvatarURL: &avatarURL,
		Provider:  "github",
	}

	mockUserService.On("GetByID", mock.Anything, userID).Return(user, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/users/me", handler.GetMe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.UserResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, userID, response.ID)
	assert.Equal(t, email, response.Email)
	assert.Equal(t, "Test User", response.Name)
	assert.Equal(t, &avatarURL, response.AvatarURL)
	assert.Equal(t, "github", response.Provider)

	mockUserService.AssertExpectations(t)
}

func TestUserHandler_GetMe_NotAuthenticated(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/users/me", handler.GetMe)

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserHandler_GetMe_UserNotFound(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	userID := uuid.New()
	email := "test@example.com"

	mockUserService.On("GetByID", mock.Anything, userID).Return(nil, errors.New("not found"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/users/me", handler.GetMe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "user not found")

	mockUserService.AssertExpectations(t)
}

func TestUserHandler_UpdateMe_Success(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	userID := uuid.New()
	email := "test@example.com"
	updatedUser := &models.User{
		ID:       userID,
		Email:    email,
		Name:     "Updated Name",
		Provider: "github",
	}

	mockUserService.On("Update", mock.Anything, userID, "Updated Name").Return(updatedUser, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/users/me", handler.UpdateMe)

	body := dto.UpdateUserRequest{Name: "Updated Name"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/users/me", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.UserResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", response.Name)

	mockUserService.AssertExpectations(t)
}

func TestUserHandler_UpdateMe_NotAuthenticated(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/users/me", handler.UpdateMe)

	body := dto.UpdateUserRequest{Name: "Updated Name"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/users/me", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserHandler_UpdateMe_EmptyName(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/users/me", handler.UpdateMe)

	body := dto.UpdateUserRequest{Name: ""}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/users/me", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name is required")
}

func TestUserHandler_UpdateMe_InvalidBody(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/users/me", handler.UpdateMe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/users/me", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request body")
}

func TestUserHandler_UpdateMe_ServiceError(t *testing.T) {
	mockUserService := new(testutil.MockUserService)
	handler := NewUserHandler(mockUserService)
	jwtSvc := newTestJWTService()

	userID := uuid.New()
	email := "test@example.com"

	mockUserService.On("Update", mock.Anything, userID, "New Name").Return(nil, errors.New("database error"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/users/me", handler.UpdateMe)

	body := dto.UpdateUserRequest{Name: "New Name"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/users/me", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to update user")

	mockUserService.AssertExpectations(t)
}
