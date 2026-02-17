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

func setupTeamTest(t *testing.T) (*testutil.MockTeamService, *testutil.MockUserService, *testutil.MockEmailService, *TeamHandler, *services.JWTService) {
	t.Helper()
	mockTeamService := new(testutil.MockTeamService)
	mockUserService := new(testutil.MockUserService)
	mockEmailService := new(testutil.MockEmailService)
	handler := NewTeamHandler(mockTeamService, mockUserService, mockEmailService, "http://localhost:8080")
	jwtSvc := services.NewJWTService("test-secret-key", 15*time.Minute, 24*time.Hour)
	return mockTeamService, mockUserService, mockEmailService, handler, jwtSvc
}

func TestTeamHandler_Create_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	team := &models.Team{
		ID:      uuid.New(),
		Name:    "My Team",
		OwnerID: userID,
	}

	mockTeamService.On("Create", mock.Anything, "My Team", userID).Return(team, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams", handler.Create)

	body := dto.CreateTeamRequest{Name: "My Team"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response dto.TeamResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, team.ID, response.ID)
	assert.Equal(t, "My Team", response.Name)
	assert.Equal(t, "owner", response.Role)

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Create_EmptyName(t *testing.T) {
	_, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams", handler.Create)

	body := dto.CreateTeamRequest{Name: ""}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name is required")
}

func TestTeamHandler_List_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teams := []models.Team{
		{ID: uuid.New(), Name: "Team 1", OwnerID: userID},
		{ID: uuid.New(), Name: "Team 2", OwnerID: uuid.New()},
	}
	roles := []string{"owner", "member"}

	mockTeamService.On("GetUserTeams", mock.Anything, userID).Return(teams, roles, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams", handler.List)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response []dto.TeamResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 2)
	assert.Equal(t, "owner", response[0].Role)
	assert.Equal(t, "member", response[1].Role)

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Get_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	team := &models.Team{
		ID:      teamID,
		Name:    "My Team",
		OwnerID: userID,
	}

	mockTeamService.On("IsMember", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("GetByID", mock.Anything, teamID).Return(team, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams/:id", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.TeamResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, teamID, response.ID)
	assert.Equal(t, "My Team", response.Name)
	assert.Equal(t, "owner", response.Role)

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Get_NotMember(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsMember", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams/:id", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "team not found")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Update_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	updatedTeam := &models.Team{
		ID:      teamID,
		Name:    "Updated Team",
		OwnerID: userID,
	}

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("Update", mock.Anything, teamID, "Updated Team").Return(updatedTeam, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/teams/:id", handler.Update)

	body := dto.UpdateTeamRequest{Name: "Updated Team"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/teams/"+teamID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.TeamResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Updated Team", response.Name)

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Update_NotOwner(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Patch("/teams/:id", handler.Update)

	body := dto.UpdateTeamRequest{Name: "Updated Team"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPatch, "/teams/"+teamID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "only owner can update team")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Delete_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("Delete", mock.Anything, teamID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "team deleted")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_Delete_NotOwner(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id", handler.Delete)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "only owner can delete team")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_GetMembers_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	avatarURL := "https://example.com/avatar.png"
	members := []models.TeamMember{
		{
			ID:     uuid.New(),
			TeamID: teamID,
			UserID: userID,
			Role:   "owner",
			User: &models.User{
				ID:        userID,
				Email:     email,
				Name:      "Test User",
				AvatarURL: &avatarURL,
				Provider:  "github",
			},
		},
	}

	mockTeamService.On("IsMember", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("GetMembers", mock.Anything, teamID).Return(members, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams/:id/members", handler.GetMembers)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamID.String()+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response []dto.TeamMemberResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 1)
	assert.Equal(t, "owner", response[0].Role)
	assert.Equal(t, email, response[0].User.Email)

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_RemoveMember_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	memberID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("RemoveMember", mock.Anything, teamID, memberID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id/members/:memberId", handler.RemoveMember)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String()+"/members/"+memberID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "member removed")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_RemoveMember_CannotRemoveSelf(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id/members/:memberId", handler.RemoveMember)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String()+"/members/"+userID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot remove yourself as owner")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_RemoveMember_CannotRemoveOwner(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()
	memberID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("RemoveMember", mock.Anything, teamID, memberID).Return(services.ErrCannotRemoveOwner)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id/members/:memberId", handler.RemoveMember)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String()+"/members/"+memberID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot remove team owner")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_LeaveTeam_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("RemoveMember", mock.Anything, teamID, userID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/leave", handler.LeaveTeam)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/leave", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "left team")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_LeaveTeam_OwnerCannotLeave(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"
	teamID := uuid.New()

	mockTeamService.On("RemoveMember", mock.Anything, teamID, userID).Return(services.ErrCannotRemoveOwner)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/leave", handler.LeaveTeam)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/leave", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "owner cannot leave team")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_InvalidTeamID(t *testing.T) {
	_, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams/:id", handler.Get)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams/invalid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid team id")
}

func TestTeamHandler_NotAuthenticated(t *testing.T) {
	_, _, _, handler, jwtSvc := setupTeamTest(t)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams", handler.List)
	app.Post("/teams", handler.Create)

	// Test List
	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Test Create
	body := dto.CreateTeamRequest{Name: "Test"}
	jsonBody, _ := json.Marshal(body)
	req = httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTeamHandler_Create_ServiceError(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "test@example.com"

	mockTeamService.On("Create", mock.Anything, "My Team", userID).Return(nil, errors.New("database error"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams", handler.Create)

	body := dto.CreateTeamRequest{Name: "My Team"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to create team")

	mockTeamService.AssertExpectations(t)
}

// InviteMember tests

func TestTeamHandler_InviteMember_Success(t *testing.T) {
	mockTeamService, mockUserService, mockEmailService, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()
	inviteeID := uuid.New()
	inviteeEmail := "invitee@example.com"
	now := time.Now()

	invitee := &models.User{
		ID:    inviteeID,
		Email: inviteeEmail,
		Name:  "Invitee User",
	}

	team := &models.Team{
		ID:      teamID,
		Name:    "Test Team",
		OwnerID: userID,
	}

	owner := &models.User{
		ID:    userID,
		Email: email,
		Name:  "Owner User",
	}

	invite := &models.TeamInvite{
		ID:        uuid.New(),
		TeamID:    teamID,
		InviterID: userID,
		InviteeID: inviteeID,
		Status:    "pending",
		CreatedAt: now,
	}

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockUserService.On("GetByEmail", mock.Anything, inviteeEmail).Return(invitee, nil)
	mockTeamService.On("CreateInvite", mock.Anything, teamID, userID, inviteeID).Return(invite, nil)
	mockTeamService.On("GetByID", mock.Anything, teamID).Return(team, nil)
	mockUserService.On("GetByID", mock.Anything, userID).Return(owner, nil)
	mockEmailService.On("SendTeamInvite", inviteeEmail, team.Name, owner.Name, mock.AnythingOfType("string")).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/invites", handler.InviteMember)

	body := dto.InviteMemberRequest{Email: inviteeEmail}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/invites", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response dto.TeamInviteResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, invite.ID, response.ID)
	assert.Equal(t, teamID, response.TeamID)

	mockTeamService.AssertExpectations(t)
	mockUserService.AssertExpectations(t)
	mockEmailService.AssertExpectations(t)
}

func TestTeamHandler_InviteMember_NotOwner(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "member@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/invites", handler.InviteMember)

	body := dto.InviteMemberRequest{Email: "invitee@example.com"}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/invites", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "only owner can invite members")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_InviteMember_UserNotFound(t *testing.T) {
	mockTeamService, mockUserService, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()
	inviteeEmail := "unknown@example.com"

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockUserService.On("GetByEmail", mock.Anything, inviteeEmail).Return(nil, errors.New("not found"))

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/invites", handler.InviteMember)

	body := dto.InviteMemberRequest{Email: inviteeEmail}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/invites", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "user with this email not found")

	mockTeamService.AssertExpectations(t)
	mockUserService.AssertExpectations(t)
}

func TestTeamHandler_InviteMember_AlreadyMember(t *testing.T) {
	mockTeamService, mockUserService, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()
	inviteeID := uuid.New()
	inviteeEmail := "member@example.com"

	invitee := &models.User{
		ID:    inviteeID,
		Email: inviteeEmail,
		Name:  "Already Member",
	}

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockUserService.On("GetByEmail", mock.Anything, inviteeEmail).Return(invitee, nil)
	mockTeamService.On("CreateInvite", mock.Anything, teamID, userID, inviteeID).Return(nil, services.ErrAlreadyMember)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/invites", handler.InviteMember)

	body := dto.InviteMemberRequest{Email: inviteeEmail}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/invites", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "user is already a team member")

	mockTeamService.AssertExpectations(t)
	mockUserService.AssertExpectations(t)
}

func TestTeamHandler_InviteMember_EmptyEmail(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/teams/:id/invites", handler.InviteMember)

	body := dto.InviteMemberRequest{Email: ""}
	jsonBody, _ := json.Marshal(body)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamID.String()+"/invites", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email is required")

	mockTeamService.AssertExpectations(t)
}

// GetTeamInvites tests

func TestTeamHandler_GetTeamInvites_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()
	now := time.Now()

	inviteeUser := &models.User{
		ID:       uuid.New(),
		Email:    "invitee@example.com",
		Name:     "Invitee",
		Provider: "github",
	}

	invites := []models.TeamInvite{
		{
			ID:        uuid.New(),
			TeamID:    teamID,
			InviterID: userID,
			InviteeID: inviteeUser.ID,
			Status:    "pending",
			CreatedAt: now,
			Invitee:   inviteeUser,
		},
	}

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("GetTeamPendingInvites", mock.Anything, teamID).Return(invites, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams/:id/invites", handler.GetTeamInvites)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamID.String()+"/invites", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response []dto.TeamInviteResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, invites[0].ID, response[0].ID)
	assert.NotNil(t, response[0].Invitee)

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_GetTeamInvites_NotOwner(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "member@example.com"
	teamID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/teams/:id/invites", handler.GetTeamInvites)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamID.String()+"/invites", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "only owner can view invites")

	mockTeamService.AssertExpectations(t)
}

// CancelInvite tests

func TestTeamHandler_CancelInvite_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()
	inviteID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("CancelInvite", mock.Anything, inviteID, teamID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id/invites/:inviteId", handler.CancelInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String()+"/invites/"+inviteID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "invite cancelled")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_CancelInvite_NotOwner(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "member@example.com"
	teamID := uuid.New()
	inviteID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(false, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id/invites/:inviteId", handler.CancelInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String()+"/invites/"+inviteID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "only owner can cancel invites")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_CancelInvite_NotFound(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "owner@example.com"
	teamID := uuid.New()
	inviteID := uuid.New()

	mockTeamService.On("IsOwner", mock.Anything, teamID, userID).Return(true, nil)
	mockTeamService.On("CancelInvite", mock.Anything, inviteID, teamID).Return(services.ErrInviteNotFound)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Delete("/teams/:id/invites/:inviteId", handler.CancelInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamID.String()+"/invites/"+inviteID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "invite not found")

	mockTeamService.AssertExpectations(t)
}

// GetMyInvites tests

func TestTeamHandler_GetMyInvites_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"
	teamID := uuid.New()
	now := time.Now()

	team := &models.Team{
		ID:      teamID,
		Name:    "Test Team",
		OwnerID: uuid.New(),
	}

	inviter := &models.User{
		ID:       uuid.New(),
		Email:    "inviter@example.com",
		Name:     "Inviter",
		Provider: "github",
	}

	invites := []models.TeamInvite{
		{
			ID:        uuid.New(),
			TeamID:    teamID,
			InviterID: inviter.ID,
			InviteeID: userID,
			Status:    "pending",
			CreatedAt: now,
			Team:      team,
			Inviter:   inviter,
		},
	}

	mockTeamService.On("GetUserPendingInvites", mock.Anything, userID).Return(invites, nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Get("/invites", handler.GetMyInvites)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodGet, "/invites", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response []dto.TeamInviteResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 1)
	assert.NotNil(t, response[0].Team)
	assert.NotNil(t, response[0].Inviter)

	mockTeamService.AssertExpectations(t)
}

// AcceptInvite tests

func TestTeamHandler_AcceptInvite_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"
	inviteID := uuid.New()

	mockTeamService.On("AcceptInvite", mock.Anything, inviteID, userID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/invites/:inviteId/accept", handler.AcceptInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/invites/"+inviteID.String()+"/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "invite accepted")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_AcceptInvite_NotFound(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"
	inviteID := uuid.New()

	mockTeamService.On("AcceptInvite", mock.Anything, inviteID, userID).Return(services.ErrInviteNotFound)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/invites/:inviteId/accept", handler.AcceptInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/invites/"+inviteID.String()+"/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "invite not found")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_AcceptInvite_InvalidID(t *testing.T) {
	_, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/invites/:inviteId/accept", handler.AcceptInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/invites/invalid-uuid/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid invite id")
}

// DeclineInvite tests

func TestTeamHandler_DeclineInvite_Success(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"
	inviteID := uuid.New()

	mockTeamService.On("DeclineInvite", mock.Anything, inviteID, userID).Return(nil)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/invites/:inviteId/decline", handler.DeclineInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/invites/"+inviteID.String()+"/decline", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "invite declined")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_DeclineInvite_NotFound(t *testing.T) {
	mockTeamService, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"
	inviteID := uuid.New()

	mockTeamService.On("DeclineInvite", mock.Anything, inviteID, userID).Return(services.ErrInviteNotFound)

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/invites/:inviteId/decline", handler.DeclineInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/invites/"+inviteID.String()+"/decline", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "invite not found")

	mockTeamService.AssertExpectations(t)
}

func TestTeamHandler_DeclineInvite_InvalidID(t *testing.T) {
	_, _, _, handler, jwtSvc := setupTeamTest(t)

	userID := uuid.New()
	email := "user@example.com"

	app := drift.New()
	app.Use(driftmw.BodyParser())
	app.Use(middleware.Auth(jwtSvc))
	app.Post("/invites/:inviteId/decline", handler.DeclineInvite)

	token := generateTestToken(t, jwtSvc, userID, email)
	req := httptest.NewRequest(http.MethodPost, "/invites/invalid-uuid/decline", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid invite id")
}
