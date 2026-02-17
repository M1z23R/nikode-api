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

func setupInviteTest(t *testing.T) (*testutil.MockTeamService, *testutil.MockUserService, *InviteHandler) {
	t.Helper()
	mockTeamService := new(testutil.MockTeamService)
	mockUserService := new(testutil.MockUserService)
	handler := NewInviteHandler(mockTeamService, mockUserService)
	return mockTeamService, mockUserService, handler
}

func TestInviteHandler_ViewInvite_Success(t *testing.T) {
	mockTeamService, mockUserService, handler := setupInviteTest(t)

	inviteID := uuid.New()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    teamID,
		InviterID: inviterID,
		InviteeID: inviteeID,
		Status:    "pending",
		CreatedAt: now,
	}

	team := &models.Team{
		ID:      teamID,
		Name:    "Test Team",
		OwnerID: inviterID,
	}

	inviter := &models.User{
		ID:    inviterID,
		Email: "inviter@example.com",
		Name:  "Inviter User",
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockTeamService.On("GetByID", mock.Anything, teamID).Return(team, nil)
	mockUserService.On("GetByID", mock.Anything, inviterID).Return(inviter, nil)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Test Team")
	assert.Contains(t, rec.Body.String(), "Inviter User")
	assert.Contains(t, rec.Body.String(), "Team Invitation")

	mockTeamService.AssertExpectations(t)
	mockUserService.AssertExpectations(t)
}

func TestInviteHandler_ViewInvite_InvalidID(t *testing.T) {
	_, _, handler := setupInviteTest(t)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/invalid-uuid", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid invite link")
}

func TestInviteHandler_ViewInvite_NotFound(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(nil, services.ErrInviteNotFound)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found or has expired")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_ViewInvite_AlreadyAccepted(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    uuid.New(),
		InviterID: uuid.New(),
		InviteeID: uuid.New(),
		Status:    "accepted",
		CreatedAt: now,
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "This invite has already been accepted")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_ViewInvite_TeamNotFound(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()
	teamID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    teamID,
		InviterID: uuid.New(),
		InviteeID: uuid.New(),
		Status:    "pending",
		CreatedAt: now,
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockTeamService.On("GetByID", mock.Anything, teamID).Return(nil, errors.New("not found"))

	app := drift.New()
	app.Get("/invite/:inviteId", handler.ViewInvite)

	req := httptest.NewRequest(http.MethodGet, "/invite/"+inviteID.String(), nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Team not found")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_AcceptInvite_Success(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()
	teamID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    teamID,
		InviterID: uuid.New(),
		InviteeID: inviteeID,
		Status:    "pending",
		CreatedAt: now,
	}

	team := &models.Team{
		ID:      teamID,
		Name:    "Test Team",
		OwnerID: uuid.New(),
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockTeamService.On("AcceptInvite", mock.Anything, inviteID, inviteeID).Return(nil)
	mockTeamService.On("GetByID", mock.Anything, teamID).Return(team, nil)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "You have joined Test Team!")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_AcceptInvite_InvalidID(t *testing.T) {
	_, _, handler := setupInviteTest(t)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/invalid-uuid/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid invite link")
}

func TestInviteHandler_AcceptInvite_NotFound(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(nil, services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_AcceptInvite_AlreadyProcessed(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    uuid.New(),
		InviterID: uuid.New(),
		InviteeID: inviteeID,
		Status:    "pending",
		CreatedAt: now,
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockTeamService.On("AcceptInvite", mock.Anything, inviteID, inviteeID).Return(services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/accept", handler.AcceptInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/accept", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found or already processed")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_DeclineInvite_Success(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    uuid.New(),
		InviterID: uuid.New(),
		InviteeID: inviteeID,
		Status:    "pending",
		CreatedAt: now,
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockTeamService.On("DeclineInvite", mock.Anything, inviteID, inviteeID).Return(nil)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite declined")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_DeclineInvite_InvalidID(t *testing.T) {
	_, _, handler := setupInviteTest(t)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/invalid-uuid/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid invite link")
}

func TestInviteHandler_DeclineInvite_NotFound(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(nil, services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found")

	mockTeamService.AssertExpectations(t)
}

func TestInviteHandler_DeclineInvite_AlreadyProcessed(t *testing.T) {
	mockTeamService, _, handler := setupInviteTest(t)

	inviteID := uuid.New()
	inviteeID := uuid.New()
	now := time.Now()

	invite := &models.TeamInvite{
		ID:        inviteID,
		TeamID:    uuid.New(),
		InviterID: uuid.New(),
		InviteeID: inviteeID,
		Status:    "pending",
		CreatedAt: now,
	}

	mockTeamService.On("GetInviteByID", mock.Anything, inviteID).Return(invite, nil)
	mockTeamService.On("DeclineInvite", mock.Anything, inviteID, inviteeID).Return(services.ErrInviteNotFound)

	app := drift.New()
	app.Post("/invite/:inviteId/decline", handler.DeclineInvite)

	req := httptest.NewRequest(http.MethodPost, "/invite/"+inviteID.String()+"/decline", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invite not found or already processed")

	mockTeamService.AssertExpectations(t)
}
