package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/google/uuid"
)

// Fixtures provides factory methods for creating test data
type Fixtures struct {
	db      *database.DB
	counter int
}

// NewFixtures creates a new fixtures factory
func NewFixtures(db *database.DB) *Fixtures {
	return &Fixtures{db: db}
}

// CreateUser creates a test user with default values
func (f *Fixtures) CreateUser(t *testing.T, opts ...UserOption) *models.User {
	t.Helper()
	f.counter++

	user := &models.User{
		Email:      fmt.Sprintf("user%d@example.com", f.counter),
		Name:       fmt.Sprintf("Test User %d", f.counter),
		Provider:   "github",
		ProviderID: fmt.Sprintf("provider-%d", f.counter),
	}

	for _, opt := range opts {
		opt(user)
	}

	ctx := context.Background()
	err := f.db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, name, avatar_url, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, name, avatar_url, provider, provider_id, created_at, updated_at
	`, user.Email, user.Name, user.AvatarURL, user.Provider, user.ProviderID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return user
}

// UserOption configures a test user
type UserOption func(*models.User)

// WithEmail sets the user's email
func WithEmail(email string) UserOption {
	return func(u *models.User) {
		u.Email = email
	}
}

// WithName sets the user's name
func WithName(name string) UserOption {
	return func(u *models.User) {
		u.Name = name
	}
}

// WithProvider sets the user's OAuth provider
func WithProvider(provider, providerID string) UserOption {
	return func(u *models.User) {
		u.Provider = provider
		u.ProviderID = providerID
	}
}

// WithAvatar sets the user's avatar URL
func WithAvatar(url string) UserOption {
	return func(u *models.User) {
		u.AvatarURL = &url
	}
}

// CreateWorkspace creates a test workspace with the given owner
func (f *Fixtures) CreateWorkspace(t *testing.T, owner *models.User, opts ...WorkspaceOption) *models.Workspace {
	t.Helper()
	f.counter++

	ws := &models.Workspace{
		Name:    fmt.Sprintf("Test Workspace %d", f.counter),
		OwnerID: owner.ID,
	}

	for _, opt := range opts {
		opt(ws)
	}

	ctx := context.Background()
	tx, err := f.db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		INSERT INTO workspaces (name, owner_id)
		VALUES ($1, $2)
		RETURNING id, name, owner_id, created_at, updated_at
	`, ws.Name, ws.OwnerID).Scan(&ws.ID, &ws.Name, &ws.OwnerID, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
	`, ws.ID, owner.ID, models.RoleOwner)
	if err != nil {
		t.Fatalf("failed to add owner as member: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	return ws
}

// WorkspaceOption configures a test workspace
type WorkspaceOption func(*models.Workspace)

// WithWorkspaceName sets the workspace name
func WithWorkspaceName(name string) WorkspaceOption {
	return func(w *models.Workspace) {
		w.Name = name
	}
}

// AddWorkspaceMember adds a member to a workspace
func (f *Fixtures) AddWorkspaceMember(t *testing.T, workspace *models.Workspace, user *models.User) {
	t.Helper()
	ctx := context.Background()

	_, err := f.db.Pool.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
	`, workspace.ID, user.ID, models.RoleMember)
	if err != nil {
		t.Fatalf("failed to add workspace member: %v", err)
	}
}

// CreateCollection creates a test collection in a workspace
func (f *Fixtures) CreateCollection(t *testing.T, workspace *models.Workspace, user *models.User, opts ...CollectionOption) *models.Collection {
	t.Helper()
	f.counter++

	col := &models.Collection{
		WorkspaceID: workspace.ID,
		Name:        fmt.Sprintf("Test Collection %d", f.counter),
		Data:        json.RawMessage(`{}`),
		UpdatedBy:   &user.ID,
	}

	for _, opt := range opts {
		opt(col)
	}

	ctx := context.Background()
	err := f.db.Pool.QueryRow(ctx, `
		INSERT INTO collections (workspace_id, name, data, updated_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, name, data, version, updated_by, created_at, updated_at
	`, col.WorkspaceID, col.Name, col.Data, col.UpdatedBy).Scan(
		&col.ID, &col.WorkspaceID, &col.Name,
		&col.Data, &col.Version, &col.UpdatedBy,
		&col.CreatedAt, &col.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("failed to create collection: %v", err)
	}

	return col
}

// CollectionOption configures a test collection
type CollectionOption func(*models.Collection)

// WithCollectionName sets the collection name
func WithCollectionName(name string) CollectionOption {
	return func(c *models.Collection) {
		c.Name = name
	}
}

// WithCollectionData sets the collection data
func WithCollectionData(data json.RawMessage) CollectionOption {
	return func(c *models.Collection) {
		c.Data = data
	}
}

// CreateRefreshToken creates a test refresh token
func (f *Fixtures) CreateRefreshToken(t *testing.T, userID uuid.UUID, tokenHash string, expiresAt time.Time) {
	t.Helper()
	ctx := context.Background()

	_, err := f.db.Pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}
}

// OAuthUserInfo creates test OAuth user info
func OAuthUserInfo(email, name, provider, id string) *oauth.UserInfo {
	return &oauth.UserInfo{
		Email:     email,
		Name:      name,
		AvatarURL: "https://example.com/avatar.png",
		ID:        id,
		Provider:  provider,
	}
}
