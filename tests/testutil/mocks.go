package testutil

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/internal/sse"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockUserService mocks the UserService
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) FindOrCreateFromOAuth(ctx context.Context, info *oauth.UserInfo) (*models.User, error) {
	args := m.Called(ctx, info)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, _ := args.Get(0).(*models.User)
	return user, args.Error(1)
}

func (m *MockUserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, _ := args.Get(0).(*models.User)
	return user, args.Error(1)
}

func (m *MockUserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, _ := args.Get(0).(*models.User)
	return user, args.Error(1)
}

func (m *MockUserService) Update(ctx context.Context, id uuid.UUID, name string) (*models.User, error) {
	args := m.Called(ctx, id, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	user, _ := args.Get(0).(*models.User)
	return user, args.Error(1)
}

// MockTeamService mocks the TeamService
type MockTeamService struct {
	mock.Mock
}

func (m *MockTeamService) Create(ctx context.Context, name string, ownerID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, name, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) GetByID(ctx context.Context, teamID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, []string, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Team), args.Get(1).([]string), args.Error(2)
}

func (m *MockTeamService) Update(ctx context.Context, teamID uuid.UUID, name string) (*models.Team, error) {
	args := m.Called(ctx, teamID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) Delete(ctx context.Context, teamID uuid.UUID) error {
	args := m.Called(ctx, teamID)
	return args.Error(0)
}

func (m *MockTeamService) IsOwner(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTeamService) IsMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTeamService) GetMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMember, error) {
	args := m.Called(ctx, teamID)
	return args.Get(0).([]models.TeamMember), args.Error(1)
}

func (m *MockTeamService) AddMember(ctx context.Context, teamID, userID uuid.UUID) error {
	args := m.Called(ctx, teamID, userID)
	return args.Error(0)
}

func (m *MockTeamService) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	args := m.Called(ctx, teamID, userID)
	return args.Error(0)
}

func (m *MockTeamService) CreateInvite(ctx context.Context, teamID, inviterID, inviteeID uuid.UUID) (*models.TeamInvite, error) {
	args := m.Called(ctx, teamID, inviterID, inviteeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TeamInvite), args.Error(1)
}

func (m *MockTeamService) GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*models.TeamInvite, error) {
	args := m.Called(ctx, inviteID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TeamInvite), args.Error(1)
}

func (m *MockTeamService) GetUserPendingInvites(ctx context.Context, userID uuid.UUID) ([]models.TeamInvite, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.TeamInvite), args.Error(1)
}

func (m *MockTeamService) GetTeamPendingInvites(ctx context.Context, teamID uuid.UUID) ([]models.TeamInvite, error) {
	args := m.Called(ctx, teamID)
	return args.Get(0).([]models.TeamInvite), args.Error(1)
}

func (m *MockTeamService) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	args := m.Called(ctx, inviteID, userID)
	return args.Error(0)
}

func (m *MockTeamService) DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	args := m.Called(ctx, inviteID, userID)
	return args.Error(0)
}

func (m *MockTeamService) CancelInvite(ctx context.Context, inviteID, teamID uuid.UUID) error {
	args := m.Called(ctx, inviteID, teamID)
	return args.Error(0)
}

// MockWorkspaceService mocks the WorkspaceService
type MockWorkspaceService struct {
	mock.Mock
}

func (m *MockWorkspaceService) Create(ctx context.Context, name string, userID uuid.UUID, teamID *uuid.UUID) (*models.Workspace, error) {
	args := m.Called(ctx, name, userID, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workspace), args.Error(1)
}

func (m *MockWorkspaceService) GetByID(ctx context.Context, workspaceID uuid.UUID) (*models.Workspace, error) {
	args := m.Called(ctx, workspaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workspace), args.Error(1)
}

func (m *MockWorkspaceService) GetUserWorkspaces(ctx context.Context, userID uuid.UUID) ([]models.Workspace, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Workspace), args.Error(1)
}

func (m *MockWorkspaceService) Update(ctx context.Context, workspaceID uuid.UUID, name string) (*models.Workspace, error) {
	args := m.Called(ctx, workspaceID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workspace), args.Error(1)
}

func (m *MockWorkspaceService) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	args := m.Called(ctx, workspaceID)
	return args.Error(0)
}

func (m *MockWorkspaceService) CanAccess(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, workspaceID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkspaceService) CanModify(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, workspaceID, userID)
	return args.Bool(0), args.Error(1)
}

// MockCollectionService mocks the CollectionService
type MockCollectionService struct {
	mock.Mock
}

func (m *MockCollectionService) Create(ctx context.Context, workspaceID uuid.UUID, name string, data json.RawMessage, userID uuid.UUID) (*models.Collection, error) {
	args := m.Called(ctx, workspaceID, name, data, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Collection), args.Error(1)
}

func (m *MockCollectionService) GetByID(ctx context.Context, collectionID uuid.UUID) (*models.Collection, error) {
	args := m.Called(ctx, collectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Collection), args.Error(1)
}

func (m *MockCollectionService) GetByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]models.Collection, error) {
	args := m.Called(ctx, workspaceID)
	return args.Get(0).([]models.Collection), args.Error(1)
}

func (m *MockCollectionService) Update(ctx context.Context, collectionID uuid.UUID, name *string, data json.RawMessage, expectedVersion int, userID uuid.UUID) (*models.Collection, error) {
	args := m.Called(ctx, collectionID, name, data, expectedVersion, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Collection), args.Error(1)
}

func (m *MockCollectionService) Delete(ctx context.Context, collectionID uuid.UUID) error {
	args := m.Called(ctx, collectionID)
	return args.Error(0)
}

// MockTokenService mocks the TokenService
type MockTokenService struct {
	mock.Mock
}

func (m *MockTokenService) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, tokenHash, expiresAt)
	return args.Error(0)
}

func (m *MockTokenService) ValidateRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	args := m.Called(ctx, tokenHash)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockTokenService) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *MockTokenService) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockTokenService) CleanupExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockOAuthProvider mocks an OAuth provider
type MockOAuthProvider struct {
	mock.Mock
}

func (m *MockOAuthProvider) GetConsentURL(state string) string {
	args := m.Called(state)
	return args.String(0)
}

func (m *MockOAuthProvider) ExchangeCode(ctx context.Context, code string) (*oauth.UserInfo, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth.UserInfo), args.Error(1)
}

func (m *MockOAuthProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

// MockJWTService mocks the JWTService
type MockJWTService struct {
	mock.Mock
}

func (m *MockJWTService) GenerateTokenPair(userID uuid.UUID, email string) (*services.TokenPair, error) {
	args := m.Called(userID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TokenPair), args.Error(1)
}

func (m *MockJWTService) ValidateRefreshToken(token string) (uuid.UUID, error) {
	args := m.Called(token)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockJWTService) RefreshExpiry() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

// MockSSEHub mocks the SSE Hub
type MockSSEHub struct {
	mock.Mock
}

func (m *MockSSEHub) Register(client *sse.Client) {
	m.Called(client)
}

func (m *MockSSEHub) Unregister(client *sse.Client) {
	m.Called(client)
}

func (m *MockSSEHub) SubscribeToWorkspace(clientID string, workspaceID uuid.UUID) {
	m.Called(clientID, workspaceID)
}

func (m *MockSSEHub) UnsubscribeFromWorkspace(clientID string, workspaceID uuid.UUID) {
	m.Called(clientID, workspaceID)
}

func (m *MockSSEHub) BroadcastCollectionUpdate(workspaceID, collectionID, userID uuid.UUID, version int) {
	m.Called(workspaceID, collectionID, userID, version)
}

// MockEmailService mocks the EmailService
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendTeamInvite(to, teamName, inviterName, inviteURL string) error {
	args := m.Called(to, teamName, inviterName, inviteURL)
	return args.Error(0)
}
