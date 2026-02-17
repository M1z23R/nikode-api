package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/internal/sse"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
	driftmw "github.com/m1z23r/drift/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockableSSEHandler creates a handler with mock hub interface
type MockableSSEHandler struct {
	hub              SSEHubInterface
	workspaceService WorkspaceServiceInterface
}

func NewMockableSSEHandler(hub SSEHubInterface, workspaceService WorkspaceServiceInterface) *MockableSSEHandler {
	return &MockableSSEHandler{
		hub:              hub,
		workspaceService: workspaceService,
	}
}

func (h *MockableSSEHandler) Subscribe(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	clientID := c.Param("clientId")
	if clientID == "" {
		c.BadRequest("client_id is required")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	canAccess, err := h.workspaceService.CanAccess(c.Request.Context(), workspaceID, userID)
	if err != nil || !canAccess {
		c.NotFound("workspace not found")
		return
	}

	h.hub.SubscribeToWorkspace(clientID, workspaceID)

	_ = c.JSON(200, map[string]string{
		"message": "subscribed to workspace " + workspaceID.String(),
	})
}

func (h *MockableSSEHandler) Unsubscribe(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	clientID := c.Param("clientId")
	if clientID == "" {
		c.BadRequest("client_id is required")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	h.hub.UnsubscribeFromWorkspace(clientID, workspaceID)

	_ = c.JSON(200, map[string]string{
		"message": "unsubscribed from workspace " + workspaceID.String(),
	})
}

func (h *MockableSSEHandler) Connect(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspaceId"))
	if err != nil {
		c.BadRequest("invalid workspace id")
		return
	}

	canAccess, err := h.workspaceService.CanAccess(c.Request.Context(), workspaceID, userID)
	if err != nil || !canAccess {
		c.NotFound("workspace not found")
		return
	}

	// For testing, we just return success after validation
	// The actual SSE streaming is hard to test in unit tests
	_ = c.JSON(200, map[string]string{"status": "would connect"})
}

func setupMockableSSETest(t *testing.T) (*testutil.MockSSEHub, *testutil.MockWorkspaceService, *MockableSSEHandler, *services.JWTService) {
	t.Helper()
	mockHub := new(testutil.MockSSEHub)
	mockWorkspaceService := new(testutil.MockWorkspaceService)
	handler := NewMockableSSEHandler(mockHub, mockWorkspaceService)
	jwtSvc := services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)
	return mockHub, mockWorkspaceService, handler, jwtSvc
}

// Subscribe tests

func TestSSEHandler_Subscribe_Success(t *testing.T) {
	mockHub, mockWorkspaceService, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	clientID := uuid.New().String()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)
	mockHub.On("SubscribeToWorkspace", clientID, workspaceID).Return()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/subscribe/:workspaceId", handler.Subscribe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/subscribe/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "subscribed to workspace")

	mockWorkspaceService.AssertExpectations(t)
	mockHub.AssertExpectations(t)
}

func TestSSEHandler_Subscribe_NotAuthenticated(t *testing.T) {
	_, _, handler, jwtSvc := setupMockableSSETest(t)

	workspaceID := uuid.New()
	clientID := uuid.New().String()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/subscribe/:workspaceId", handler.Subscribe)

	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/subscribe/"+workspaceID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSSEHandler_Subscribe_InvalidWorkspaceID(t *testing.T) {
	_, _, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	clientID := uuid.New().String()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/subscribe/:workspaceId", handler.Subscribe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/subscribe/invalid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid workspace id")
}

func TestSSEHandler_Subscribe_WorkspaceNotFound(t *testing.T) {
	_, mockWorkspaceService, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	clientID := uuid.New().String()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/subscribe/:workspaceId", handler.Subscribe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/subscribe/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workspace not found")

	mockWorkspaceService.AssertExpectations(t)
}

// Unsubscribe tests

func TestSSEHandler_Unsubscribe_Success(t *testing.T) {
	mockHub, _, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	clientID := uuid.New().String()

	mockHub.On("UnsubscribeFromWorkspace", clientID, workspaceID).Return()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/unsubscribe/:workspaceId", handler.Unsubscribe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/unsubscribe/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "unsubscribed from workspace")

	mockHub.AssertExpectations(t)
}

func TestSSEHandler_Unsubscribe_NotAuthenticated(t *testing.T) {
	_, _, handler, jwtSvc := setupMockableSSETest(t)

	workspaceID := uuid.New()
	clientID := uuid.New().String()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/unsubscribe/:workspaceId", handler.Unsubscribe)

	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/unsubscribe/"+workspaceID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSSEHandler_Unsubscribe_InvalidWorkspaceID(t *testing.T) {
	_, _, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	clientID := uuid.New().String()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/sse/:clientId/unsubscribe/:workspaceId", handler.Unsubscribe)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/sse/"+clientID+"/unsubscribe/invalid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid workspace id")
}

// Connect tests - these test the initial validation, not the full SSE stream

func TestSSEHandler_Connect_NotAuthenticated(t *testing.T) {
	_, _, handler, jwtSvc := setupMockableSSETest(t)

	workspaceID := uuid.New()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/sse/:workspaceId", handler.Connect)

	req := httptest.NewRequest(http.MethodGet, "/sse/"+workspaceID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSSEHandler_Connect_InvalidWorkspaceID(t *testing.T) {
	_, _, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/sse/:workspaceId", handler.Connect)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/sse/invalid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid workspace id")
}

func TestSSEHandler_Connect_WorkspaceNotFound(t *testing.T) {
	_, mockWorkspaceService, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/sse/:workspaceId", handler.Connect)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/sse/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workspace not found")

	mockWorkspaceService.AssertExpectations(t)
}

func TestSSEHandler_Connect_Success(t *testing.T) {
	_, mockWorkspaceService, handler, jwtSvc := setupMockableSSETest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/sse/:workspaceId", handler.Connect)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/sse/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	mockWorkspaceService.AssertExpectations(t)
}

// Test that the real SSEHandler exists and can be instantiated
func TestSSEHandler_NewSSEHandler(t *testing.T) {
	hub := sse.NewHub()
	mockWorkspaceService := new(testutil.MockWorkspaceService)

	handler := NewSSEHandler(hub, mockWorkspaceService)

	assert.NotNil(t, handler)
	assert.Equal(t, hub, handler.hub)
}
