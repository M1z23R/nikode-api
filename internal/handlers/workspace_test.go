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

func setupWorkspaceTest(t *testing.T) (*testutil.MockWorkspaceService, *testutil.MockTeamService, *WorkspaceHandler, *services.JWTService) {
	t.Helper()
	mockWorkspaceService := new(testutil.MockWorkspaceService)
	mockTeamService := new(testutil.MockTeamService)
	handler := NewWorkspaceHandler(mockWorkspaceService, mockTeamService)
	jwtSvc := services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)
	return mockWorkspaceService, mockTeamService, handler, jwtSvc
}

func TestWorkspaceHandler_Create_Personal_Success(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspace := &models.Workspace{
		ID:     uuid.New(),
		Name:   "My Workspace",
		UserID: &userID,
		TeamID: nil,
	}

	mockWorkspaceService.On("Create", mock.Anything, "My Workspace", userID, (*uuid.UUID)(nil)).Return(workspace, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces", handler.Create)

	body := dto.CreateWorkspaceRequest{Name: "My Workspace"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response dto.WorkspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, workspace.ID, response.ID)
	assert.Equal(t, "My Workspace", response.Name)
	assert.Equal(t, "personal", response.Type)
	assert.Nil(t, response.TeamID)

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Create_Team_Success(t *testing.T) {
	mockWorkspaceService, mockTeamService, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	workspace := &models.Workspace{
		ID:     uuid.New(),
		Name:   "Team Workspace",
		UserID: &userID,
		TeamID: &teamID,
	}

	mockTeamService.On("IsMember", mock.Anything, teamID, userID).Return(true, nil)
	mockWorkspaceService.On("Create", mock.Anything, "Team Workspace", userID, &teamID).Return(workspace, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces", handler.Create)

	body := dto.CreateWorkspaceRequest{Name: "Team Workspace", TeamID: &teamID}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response dto.WorkspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "team", response.Type)
	assert.Equal(t, &teamID, response.TeamID)

	mockWorkspaceService.AssertExpectations(t)
	mockTeamService.AssertExpectations(t)
}

func TestWorkspaceHandler_Create_Team_NotMember(t *testing.T) {
	_, mockTeamService, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsMember", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces", handler.Create)

	body := dto.CreateWorkspaceRequest{Name: "Team Workspace", TeamID: &teamID}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "not a member of this team")

	mockTeamService.AssertExpectations(t)
}

func TestWorkspaceHandler_Create_EmptyName(t *testing.T) {
	_, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/workspaces", handler.Create)

	body := dto.CreateWorkspaceRequest{Name: ""}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name is required")
}

func TestWorkspaceHandler_List_Success(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	workspaces := []models.Workspace{
		{ID: uuid.New(), Name: "Personal Workspace", UserID: &userID, TeamID: nil},
		{ID: uuid.New(), Name: "Team Workspace", UserID: &userID, TeamID: &teamID},
	}

	mockWorkspaceService.On("GetUserWorkspaces", mock.Anything, userID).Return(workspaces, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces", handler.List)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response []dto.WorkspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 2)
	assert.Equal(t, "personal", response[0].Type)
	assert.Equal(t, "team", response[1].Type)

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Get_Success(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	workspace := &models.Workspace{
		ID:     workspaceID,
		Name:   "My Workspace",
		UserID: &userID,
		TeamID: nil,
	}

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(true, nil)
	mockWorkspaceService.On("GetByID", mock.Anything, workspaceID).Return(workspace, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.WorkspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, workspaceID, response.ID)
	assert.Equal(t, "My Workspace", response.Name)

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Get_NotFound(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanAccess", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workspace not found")

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Get_InvalidID(t *testing.T) {
	_, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces/:workspaceId", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/workspaces/invalid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid workspace id")
}

func TestWorkspaceHandler_Update_Success(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()
	updatedWorkspace := &models.Workspace{
		ID:     workspaceID,
		Name:   "Updated Name",
		UserID: &userID,
		TeamID: nil,
	}

	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(true, nil)
	mockWorkspaceService.On("Update", mock.Anything, workspaceID, "Updated Name").Return(updatedWorkspace, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/workspaces/:workspaceId", handler.Update)

	body := dto.UpdateWorkspaceRequest{Name: "Updated Name"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+workspaceID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.WorkspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", response.Name)

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Update_Forbidden(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/workspaces/:workspaceId", handler.Update)

	body := dto.UpdateWorkspaceRequest{Name: "Updated Name"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+workspaceID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot modify this workspace")

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Delete_Success(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(true, nil)
	mockWorkspaceService.On("Delete", mock.Anything, workspaceID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/workspaces/:workspaceId", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "workspace deleted")

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Delete_Forbidden(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/workspaces/:workspaceId", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot delete this workspace")

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_Delete_ServiceError(t *testing.T) {
	mockWorkspaceService, _, handler, jwtSvc := setupWorkspaceTest(t)

	userID := uuid.New()
	email := "test@example.com"
	workspaceID := uuid.New()

	mockWorkspaceService.On("CanModify", mock.Anything, workspaceID, userID).Return(true, nil)
	mockWorkspaceService.On("Delete", mock.Anything, workspaceID).Return(errors.New("database error"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/workspaces/:workspaceId", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+workspaceID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to delete workspace")

	mockWorkspaceService.AssertExpectations(t)
}

func TestWorkspaceHandler_NotAuthenticated(t *testing.T) {
	_, _, handler, jwtSvc := setupWorkspaceTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/workspaces", handler.List)
	app.Post("/workspaces", handler.Create)

	// Test List
	req := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Test Create
	body := dto.CreateWorkspaceRequest{Name: "Test"}
	jsonBody, _ := json.Marshal(body)
	req = httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
