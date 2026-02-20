package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
)

var (
	ErrAPIKeyNotFound  = errors.New("api key not found")
	ErrAPIKeyRevoked   = errors.New("api key has been revoked")
	ErrAPIKeyExpired   = errors.New("api key has expired")
	ErrAPIKeyInvalid   = errors.New("invalid api key")
)

const (
	apiKeyPrefix    = "nik_"
	apiKeyRandomLen = 32
)

type APIKeyService struct {
	db *database.DB
}

func NewAPIKeyService(db *database.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// GenerateAPIKey generates a new API key with the format: nik_<workspace_prefix_8chars>_<32_random_bytes_base62>
func (s *APIKeyService) GenerateAPIKey(workspaceID uuid.UUID) (plainKey, keyHash, keyPrefix string) {
	// Get first 8 chars of workspace ID (without hyphens)
	workspacePrefix := workspaceID.String()[:8]
	workspacePrefix = workspacePrefix[:4] + workspacePrefix[5:8] // Remove the hyphen

	// Generate random bytes
	randomBytes := make([]byte, apiKeyRandomLen)
	_, _ = rand.Read(randomBytes)
	randomPart := hex.EncodeToString(randomBytes)

	// Construct the full key
	plainKey = apiKeyPrefix + workspacePrefix + "_" + randomPart
	keyPrefix = apiKeyPrefix + workspacePrefix + "..."

	// Hash the key for storage
	hash := sha256.Sum256([]byte(plainKey))
	keyHash = hex.EncodeToString(hash[:])

	return plainKey, keyHash, keyPrefix
}

// Create creates a new API key for a workspace
func (s *APIKeyService) Create(ctx context.Context, workspaceID uuid.UUID, name string, createdBy uuid.UUID, expiresAt *time.Time) (*models.WorkspaceAPIKey, string, error) {
	plainKey, keyHash, keyPrefix := s.GenerateAPIKey(workspaceID)

	var apiKey models.WorkspaceAPIKey
	err := s.db.Pool.QueryRow(ctx, `
		INSERT INTO workspace_api_keys (workspace_id, name, key_hash, key_prefix, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, name, key_hash, key_prefix, created_by, expires_at, revoked_at, last_used_at, created_at
	`, workspaceID, name, keyHash, keyPrefix, createdBy, expiresAt).Scan(
		&apiKey.ID, &apiKey.WorkspaceID, &apiKey.Name, &apiKey.KeyHash,
		&apiKey.KeyPrefix, &apiKey.CreatedBy, &apiKey.ExpiresAt,
		&apiKey.RevokedAt, &apiKey.LastUsedAt, &apiKey.CreatedAt,
	)
	if err != nil {
		return nil, "", err
	}

	return &apiKey, plainKey, nil
}

// ValidateAndGetWorkspace validates an API key and returns the workspace ID
func (s *APIKeyService) ValidateAndGetWorkspace(ctx context.Context, key string) (uuid.UUID, error) {
	// Hash the provided key
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	var apiKey models.WorkspaceAPIKey
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, expires_at, revoked_at
		FROM workspace_api_keys
		WHERE key_hash = $1
	`, keyHash).Scan(&apiKey.ID, &apiKey.WorkspaceID, &apiKey.ExpiresAt, &apiKey.RevokedAt)
	if err != nil {
		return uuid.Nil, ErrAPIKeyInvalid
	}

	// Check if revoked
	if apiKey.RevokedAt != nil {
		return uuid.Nil, ErrAPIKeyRevoked
	}

	// Check if expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return uuid.Nil, ErrAPIKeyExpired
	}

	// Update last_used_at asynchronously
	go func() {
		_, _ = s.db.Pool.Exec(context.Background(), `
			UPDATE workspace_api_keys SET last_used_at = NOW() WHERE id = $1
		`, apiKey.ID)
	}()

	return apiKey.WorkspaceID, nil
}

// List returns all API keys for a workspace (excluding revoked ones by default)
func (s *APIKeyService) List(ctx context.Context, workspaceID uuid.UUID) ([]models.WorkspaceAPIKey, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, workspace_id, name, key_hash, key_prefix, created_by, expires_at, revoked_at, last_used_at, created_at
		FROM workspace_api_keys
		WHERE workspace_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.WorkspaceAPIKey
	for rows.Next() {
		var k models.WorkspaceAPIKey
		if err := rows.Scan(
			&k.ID, &k.WorkspaceID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.CreatedBy, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt, &k.CreatedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// Revoke revokes an API key
func (s *APIKeyService) Revoke(ctx context.Context, keyID, workspaceID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		UPDATE workspace_api_keys
		SET revoked_at = NOW()
		WHERE id = $1 AND workspace_id = $2 AND revoked_at IS NULL
	`, keyID, workspaceID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

// CleanupExpired removes expired API keys (can be called periodically)
func (s *APIKeyService) CleanupExpired(ctx context.Context) error {
	_, err := s.db.Pool.Exec(ctx, `
		DELETE FROM workspace_api_keys
		WHERE expires_at < NOW() OR revoked_at IS NOT NULL AND revoked_at < NOW() - INTERVAL '30 days'
	`)
	return err
}
