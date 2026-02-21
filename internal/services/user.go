package services

import (
	"context"
	"fmt"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/google/uuid"
)

type UserService struct {
	db *database.DB
}

func NewUserService(db *database.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) FindOrCreateFromOAuth(ctx context.Context, info *oauth.UserInfo) (*models.User, error) {
	var user models.User
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, provider, provider_id, global_role, created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
	`, info.Provider, info.ID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.GlobalRole, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == nil {
		if user.Email != info.Email || user.Name != info.Name || (user.AvatarURL == nil && info.AvatarURL != "") {
			_, _ = s.db.Pool.Exec(ctx, `
				UPDATE users SET email = $1, name = $2, avatar_url = $3, updated_at = NOW()
				WHERE id = $4
			`, info.Email, info.Name, nullableString(info.AvatarURL), user.ID)
			user.Email = info.Email
			user.Name = info.Name
			if info.AvatarURL != "" {
				user.AvatarURL = &info.AvatarURL
			}
		}
		return &user, nil
	}

	err = s.db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, name, avatar_url, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, name, avatar_url, provider, provider_id, global_role, created_at, updated_at
	`, info.Email, info.Name, nullableString(info.AvatarURL), info.Provider, info.ID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.GlobalRole, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, provider, provider_id, global_role, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.GlobalRole, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, provider, provider_id, global_role, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.GlobalRole, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, name string) (*models.User, error) {
	var user models.User
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE users SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, email, name, avatar_url, provider, provider_id, global_role, created_at, updated_at
	`, name, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.GlobalRole, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
