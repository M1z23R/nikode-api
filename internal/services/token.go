package services

import (
	"context"
	"time"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/google/uuid"
)

type TokenService struct {
	db *database.DB
}

func NewTokenService(db *database.DB) *TokenService {
	return &TokenService{db: db}
}

func (s *TokenService) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (s *TokenService) ValidateRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := s.db.Pool.QueryRow(ctx, `
		SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
	`, tokenHash).Scan(&userID)
	return userID, err
}

func (s *TokenService) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *TokenService) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	return err
}

func (s *TokenService) CleanupExpired(ctx context.Context) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW()`)
	return err
}
