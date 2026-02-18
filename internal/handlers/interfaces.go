package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/internal/sse"
	"github.com/google/uuid"
)

// UserServiceInterface defines the methods used by handlers from UserService
type UserServiceInterface interface {
	FindOrCreateFromOAuth(ctx context.Context, info *oauth.UserInfo) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, name string) (*models.User, error)
}

// WorkspaceServiceInterface defines the methods used by handlers from WorkspaceService
type WorkspaceServiceInterface interface {
	Create(ctx context.Context, name string, ownerID uuid.UUID) (*models.Workspace, error)
	GetByID(ctx context.Context, workspaceID uuid.UUID) (*models.Workspace, error)
	GetUserWorkspaces(ctx context.Context, userID uuid.UUID) ([]models.Workspace, []string, error)
	Update(ctx context.Context, workspaceID uuid.UUID, name string) (*models.Workspace, error)
	Delete(ctx context.Context, workspaceID uuid.UUID) error
	IsOwner(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error)
	IsMember(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error)
	CanAccess(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error)
	CanModify(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error)
	GetMembers(ctx context.Context, workspaceID uuid.UUID) ([]models.WorkspaceMember, error)
	AddMember(ctx context.Context, workspaceID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error
	CreateInvite(ctx context.Context, workspaceID, inviterID, inviteeID uuid.UUID) (*models.WorkspaceInvite, error)
	GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*models.WorkspaceInvite, error)
	GetInviteWithDetails(ctx context.Context, inviteID uuid.UUID) (*models.WorkspaceInvite, error)
	GetUserPendingInvites(ctx context.Context, userID uuid.UUID) ([]models.WorkspaceInvite, error)
	GetWorkspacePendingInvites(ctx context.Context, workspaceID uuid.UUID) ([]models.WorkspaceInvite, error)
	AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error
	DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error
	CancelInvite(ctx context.Context, inviteID, workspaceID uuid.UUID) error
}

// CollectionServiceInterface defines the methods used by handlers from CollectionService
type CollectionServiceInterface interface {
	Create(ctx context.Context, workspaceID uuid.UUID, name string, data json.RawMessage, userID uuid.UUID) (*models.Collection, error)
	GetByID(ctx context.Context, collectionID uuid.UUID) (*models.Collection, error)
	GetByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]models.Collection, error)
	Update(ctx context.Context, collectionID uuid.UUID, name *string, data json.RawMessage, expectedVersion int, userID uuid.UUID) (*models.Collection, error)
	Delete(ctx context.Context, collectionID uuid.UUID) error
}

// TokenServiceInterface defines the methods used by handlers from TokenService
type TokenServiceInterface interface {
	StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	ValidateRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// JWTServiceInterface defines the methods used by handlers from JWTService
type JWTServiceInterface interface {
	GenerateTokenPair(userID uuid.UUID, email string) (*services.TokenPair, error)
	ValidateRefreshToken(token string) (uuid.UUID, error)
	RefreshExpiry() time.Duration
}

// SSEHubInterface defines the methods used by handlers from SSE Hub
type SSEHubInterface interface {
	Register(client *sse.Client)
	Unregister(client *sse.Client)
	SubscribeToWorkspace(clientID string, workspaceID uuid.UUID)
	UnsubscribeFromWorkspace(clientID string, workspaceID uuid.UUID)
	BroadcastCollectionUpdate(workspaceID, collectionID, userID uuid.UUID, version int)
}

// EmailServiceInterface defines the methods used by handlers from EmailService
type EmailServiceInterface interface {
	SendWorkspaceInvite(to, workspaceName, inviterName, inviteURL string) error
}
