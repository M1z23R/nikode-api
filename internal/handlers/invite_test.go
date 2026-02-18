package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/tests/testutil"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupInviteTest(t *testing.T) (*testutil.MockWorkspaceService, *InviteHandler) {
	t.Helper()
	mockWorkspaceService := new(testutil.MockWorkspaceService)
	handler := NewInviteHandler(mockWorkspaceService)
	return mockWorkspaceService, handler
}

func TestInviteHandler_ViewInvite_Success(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	workspaceID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	workspace := &models.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		OwnerID: inviterID,
	}

	inviter := &models.User{
		ID:    inviterID,
		Email: "inviter@example.com",
		Name:  "Inviter User",
	}

	invite := &models.WorkspaceInvite{
		ID:          inviteID,
		WorkspaceID: workspaceID,
		InviterID:   inviterID,
		InviteeID:   inviteeID,
		Status:      "pending",
		CreatedAt:   now,
		Workspace:   workspace,
		Inviter:     inviter,
	}

	mockWorkspaceService.On("GetInviteWithDetails", mock.Anything, inviteID).Return(invite, nil)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Test Workspace")
	assert.Contains(t, rec.Body.String(), "Inviter User")
	assert.Contains(t, rec.Body.String(), "Workspace Invitation")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_ViewInvite_InvalidID(t *testing.T) {
	_, handler := setupInviteTest(t)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/invalid-uuid", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid invite link")
}

func TestInviteHandler_ViewInvite_NotFound(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()

	mockWorkspaceService.On("GetInviteWithDetails", mock.Anything, inviteID).Return(nil, services.ErrInviteNotFound)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found or has expired")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_ViewInvite_AlreadyAccepted(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	now := time.Now()

	invite := &models.WorkspaceInvite{
		ID:          inviteID,
		WorkspaceID: uuid.New(),
		InviterID:   uuid.New(),
		InviteeID:   uuid.New(),
		Status:      "accepted",
		CreatedAt:   now,
	}

	mockWorkspaceService.On("GetInviteWithDetails", mock.Anything, inviteID).Return(invite, nil)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "This invite has already been accepted")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_AcceptInvite_Success(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	workspaceID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.WorkspaceInvite{
		ID:          inviteID,
		WorkspaceID: workspaceID,
		InviterID:   uuid.New(),
		InviteeID:   inviteeID,
		Status:      "pending",
		CreatedAt:   now,
	}

	workspace := &models.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		OwnerID: uuid.New(),
	}

	mockWorkspaceService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockWorkspaceService.On("AcceptInvite", mock.Anything, inviteID, inviteeID).Return(nil)
	mockWorkspaceService.On("GetByID", mock.Anything, workspaceID).Return(workspace, nil)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "You have joined Test Workspace!")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_AcceptInvite_InvalidID(t *testing.T) {
	_, handler := setupInviteTest(t)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/invalid-uuid/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid invite link")
}

func TestInviteHandler_AcceptInvite_NotFound(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()

	mockWorkspaceService.On("GetInviteByID", mock.Anything, inviteID).Return(nil, services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_AcceptInvite_AlreadyProcessed(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.WorkspaceInvite{
		ID:          inviteID,
		WorkspaceID: uuid.New(),
		InviterID:   uuid.New(),
		InviteeID:   inviteeID,
		Status:      "pending",
		CreatedAt:   now,
	}

	mockWorkspaceService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockWorkspaceService.On("AcceptInvite", mock.Anything, inviteID, inviteeID).Return(services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found or already processed")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_DeclineInvite_Success(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.WorkspaceInvite{
		ID:          inviteID,
		WorkspaceID: uuid.New(),
		InviterID:   uuid.New(),
		InviteeID:   inviteeID,
		Status:      "pending",
		CreatedAt:   now,
	}

	mockWorkspaceService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockWorkspaceService.On("DeclineInvite", mock.Anything, inviteID, inviteeID).Return(nil)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite declined")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_DeclineInvite_InvalidID(t *testing.T) {
	_, handler := setupInviteTest(t)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/invalid-uuid/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid invite link")
}

func TestInviteHandler_DeclineInvite_NotFound(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()

	mockWorkspaceService.On("GetInviteByID", mock.Anything, inviteID).Return(nil, services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found")

	mockWorkspaceService.AssertExpectations(t)
}

func TestInviteHandler_DeclineInvite_AlreadyProcessed(t *testing.T) {
	mockWorkspaceService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.WorkspaceInvite{
		ID:          inviteID,
		WorkspaceID: uuid.New(),
		InviterID:   uuid.New(),
		InviteeID:   inviteeID,
		Status:      "pending",
		CreatedAt:   now,
	}

	mockWorkspaceService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockWorkspaceService.On("DeclineInvite", mock.Anything, inviteeID).Return(errors.New("something went wrong"))

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	// Note: The current implementation doesn't check for specific errors here
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	mockWorkspaceService.AssertExpectations(t)
}
