package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/dimitrije/nikode-api/internal/services"
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
	GetByWorkspaceAndName(ctx context.Context, workspaceID uuid.UUID, name string) (*models.Collection, error)
	Update(ctx context.Context, collectionID uuid.UUID, name *string, data json.RawMessage, expectedVersion int, userID uuid.UUID) (*models.Collection, error)
	ForceUpdate(ctx context.Context, collectionID uuid.UUID, name string, data json.RawMessage) (*models.Collection, error)
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
	GenerateTokenPair(userID uuid.UUID, email, globalRole string) (*services.TokenPair, error)
	ValidateRefreshToken(token string) (uuid.UUID, error)
	RefreshExpiry() time.Duration
}

// HubInterface defines the methods used by handlers from the Hub
type HubInterface interface {
	Register(client *hub.Client)
	Unregister(client *hub.Client)
	SubscribeToWorkspace(clientID string, workspaceID uuid.UUID)
	UnsubscribeFromWorkspace(clientID string, workspaceID uuid.UUID)
	BroadcastCollectionCreate(workspaceID, collectionID, createdBy uuid.UUID, name string, version int)
	BroadcastCollectionUpdate(workspaceID, collectionID, updatedBy uuid.UUID, name string, version int)
	BroadcastCollectionDelete(workspaceID, collectionID, deletedBy uuid.UUID)
	BroadcastWorkspaceUpdate(workspaceID, updatedBy uuid.UUID, name string)
	BroadcastMemberJoined(workspaceID, userID uuid.UUID, userName string, avatarURL *string)
	BroadcastMemberLeft(workspaceID, userID uuid.UUID)
	BroadcastToUser(userID uuid.UUID, eventType string, data any)

	// Chat
	SendChatMessage(workspaceID, senderID uuid.UUID, senderName string, avatarURL *string, content string, encrypted bool) (*hub.ChatMessage, error)
	GetChatHistory(workspaceID uuid.UUID, limit int) []hub.ChatMessage
	IsSubscribedToWorkspace(clientID string, workspaceID uuid.UUID) bool

	// Key exchange
	SetPublicKey(clientID string, publicKey string)
	GetWorkspacePublicKeys(workspaceID uuid.UUID) []hub.PublicKeyInfo
	RelayEncryptedKey(targetUserID uuid.UUID, fromUserID uuid.UUID, workspaceID uuid.UUID, encryptedKey string)
}

// EmailServiceInterface defines the methods used by handlers from EmailService
type EmailServiceInterface interface {
	SendWorkspaceInvite(to, workspaceName, inviterName, inviteURL string) error
}

// APIKeyServiceInterface defines the methods used by handlers from APIKeyService
type APIKeyServiceInterface interface {
	Create(ctx context.Context, workspaceID uuid.UUID, name string, createdBy uuid.UUID, expiresAt *time.Time) (*models.WorkspaceAPIKey, string, error)
	ValidateAndGetWorkspace(ctx context.Context, key string) (uuid.UUID, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]models.WorkspaceAPIKey, error)
	Revoke(ctx context.Context, keyID, workspaceID uuid.UUID) error
}

// VaultServiceInterface defines the methods used by handlers from VaultService
type VaultServiceInterface interface {
	Create(ctx context.Context, workspaceID uuid.UUID, salt, verification string) (*models.Vault, error)
	GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (*models.Vault, error)
	Delete(ctx context.Context, workspaceID uuid.UUID) error
	ListItems(ctx context.Context, vaultID uuid.UUID) ([]models.VaultItem, error)
	CreateItem(ctx context.Context, vaultID uuid.UUID, data string) (*models.VaultItem, error)
	UpdateItem(ctx context.Context, itemID, vaultID uuid.UUID, data string) (*models.VaultItem, error)
	DeleteItem(ctx context.Context, itemID, vaultID uuid.UUID) error
}

// OpenAPIServiceInterface defines the methods used by handlers from OpenAPIService
type OpenAPIServiceInterface interface {
	ParseOpenAPI(content []byte) (any, error)
	ConvertToNikode(spec any) (json.RawMessage, error)
}

// TemplateServiceInterface defines the methods used by handlers from TemplateService
type TemplateServiceInterface interface {
	Search(ctx context.Context, query string, limit int) ([]models.PublicTemplate, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.PublicTemplate, error)
	Create(ctx context.Context, name, description, category string, data []byte) (*models.PublicTemplate, error)
}
