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

func setupCollectionTest(t *testing.T) (*testutil.MockCollectionService, *testutil.MockWorkspaceService, *testutil.MockSSEHub, *CollectionHandler, *services.JWTService) {
	t.Helper()
	mockCollectionService := new(testutil.MockCollectionService)
	mockWorkspaceService := new(testutil.MockWorkspaceService)
	mockHub := new(testutil.MockSSEHub)
	handler := NewCollectionHandler(mockCollectionService, mockWorkspaceService, mockHub)
	jwtSvc := services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)
	return mockCollectionService, mockWorkspaceService, mockHub, handler, jwtSvc
}

func TestCollectionHandler_Create_Success(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionData := json.RawMessage(`{"key": "value"}`)
	collection := &models.Collection{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        "My Collection",
		Data:        collectionData,
		Version:     1,
		UpdatedBy:   &userID,
	}

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)
	mockCollectionService.On("Create", mock.Anything, workspaceID, "My Collection", mock.Anything, userID).Return(collection, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces/:workspaceId/collections", handler.Create)

	body := dto.CreateCollectionRequest{Name: "My Collection", Data: collectionData}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID.String()+"/collections", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response dto.CollectionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, collection.ID, response.ID)
	assert.Equal(t, "My Collection", response.Name)
	assert.Equal(t, 1, response.Version)

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Create_WorkspaceNotFound(t *testing.T) {
	_, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces/:workspaceId/collections", handler.Create)

	body := dto.CreateCollectionRequest{Name: "My Collection"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID.String()+"/collections", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workspace not found")

	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Create_EmptyName(t *testing.T) {
	_, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces/:workspaceId/collections", handler.Create)

	body := dto.CreateCollectionRequest{Name: ""}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID.String()+"/collections", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name is required")

	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_List_Success(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collections := []models.Collection{
		{ID: uuid.New(), WorkspaceID: workspaceID, Name: "Collection 1", Version: 1, UpdatedBy: &userID},
		{ID: uuid.New(), WorkspaceID: workspaceID, Name: "Collection 2", Version: 2, UpdatedBy: &userID},
	}

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)
	mockCollectionService.On("GetByWorkspace", mock.Anything, workspaceID).Return(collections, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId/collections", handler.List)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String()+"/collections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response []dto.CollectionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 2)
	assert.Equal(t, "Collection 1", response[0].Name)
	assert.Equal(t, "Collection 2", response[1].Name)

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Get_Success(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()
	collection := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "My Collection",
		Version:     1,
		UpdatedBy:   &userID,
	}

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(collection, nil)
	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId/collections/:collectionId", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.CollectionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, collectionID, response.ID)
	assert.Equal(t, "My Collection", response.Name)

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Get_NotFound(t *testing.T) {
	mockCollectionService, _, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(nil, errors.New("not found"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId/collections/:collectionId", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "collection not found")

	mockCollectionService.AssertExpectations(t)
}

func TestCollectionHandler_Update_Success(t *testing.T) {
	mockCollectionService, mockWorkspaceService, mockHub, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()
	newName := "Updated Name"
	newData := json.RawMessage(`{"updated": true}`)

	existing := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Old Name",
		Version:     1,
		UpdatedBy:   &userID,
	}

	updated := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        newName,
		Data:        newData,
		Version:     2,
		UpdatedBy:   &userID,
	}

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(existing, nil)
	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)
	mockCollectionService.On("Update", mock.Anything, collectionID, &newName, mock.Anything, 1, userID).Return(updated, nil)
	mockHub.On("BroadcastCollectionUpdate", workspaceID, collectionID, userID, 2).Return()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/workspaces/:workspaceId/collections/:collectionId", handler.Update)

	body := dto.UpdateCollectionRequest{Name: &newName, Data: newData, Version: 1}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.CollectionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", response.Name)
	assert.Equal(t, 2, response.Version)

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
	mockHub.AssertExpectations(t)
}

func TestCollectionHandler_Update_VersionConflict(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()
	newName := "Updated Name"

	existing := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Old Name",
		Version:     3,
		UpdatedBy:   &userID,
	}

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(existing, nil)
	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)
	mockCollectionService.On("Update", mock.Anything, collectionID, &newName, mock.Anything, 1, userID).Return(nil, services.ErrVersionConflict)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/workspaces/:workspaceId/collections/:collectionId", handler.Update)

	body := dto.UpdateCollectionRequest{Name: &newName, Version: 1}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "VERSION_CONFLICT", response["code"])
	assert.Equal(t, float64(3), response["current_version"])

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Update_MissingVersion(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()
	newName := "Updated Name"

	existing := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Old Name",
		Version:     1,
		UpdatedBy:   &userID,
	}

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(existing, nil)
	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/workspaces/:workspaceId/collections/:collectionId", handler.Update)

	body := dto.UpdateCollectionRequest{Name: &newName, Version: 0}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "version is required")

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Delete_Success(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()

	collection := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "My Collection",
		Version:     1,
	}

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(collection, nil)
	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(true, nil)
	mockCollectionService.On("Delete", mock.Anything, collectionID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/workspaces/:workspaceId/collections/:collectionId", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "collection deleted")

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_Delete_Forbidden(t *testing.T) {
	mockCollectionService, mockWorkspaceService, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	collectionID := uuid.New()

	collection := &models.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "My Collection",
		Version:     1,
	}

	mockCollectionService.On("GetByID", mock.Anything, collectionID).Return(collection, nil)
	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/workspaces/:workspaceId/collections/:collectionId", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+workspaceID.String()+"/collections/"+collectionID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot delete this collection")

	mockCollectionService.AssertExpectations(t)
	mockWorkspaceService.AssertExpectations(t)
}

func TestCollectionHandler_InvalidWorkspaceID(t *testing.T) {
	_, _, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId/collections", handler.List)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/invalid-uuid/collections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid workspace id")
}

func TestCollectionHandler_InvalidCollectionID(t *testing.T) {
	_, _, _, handler, jwtSvc := setupCollectionTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId/collections/:collectionId", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String()+"/collections/invalid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid collection id")
}
