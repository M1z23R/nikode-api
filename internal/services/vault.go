package services

import (
	"context"
	"errors"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrVaultNotFound      = errors.New("vault not found")
	ErrVaultAlreadyExists = errors.New("vault already exists for this workspace")
	ErrVaultItemNotFound  = errors.New("vault item not found")
)

type VaultService struct {
	db *database.DB
}

func NewVaultService(db *database.DB) *VaultService {
	return &VaultService{db: db}
}

func (s *VaultService) Create(ctx context.Context, workspaceID uuid.UUID, salt, verification string) (*models.Vault, error) {
	var vault models.Vault
	err := s.db.Pool.QueryRow(ctx, `
		INSERT INTO workspace_vaults (workspace_id, salt, verification)
		VALUES ($1, $2, $3)
		RETURNING id, workspace_id, salt, verification, created_at, updated_at
	`, workspaceID, salt, verification).Scan(
		&vault.ID, &vault.WorkspaceID, &vault.Salt, &vault.Verification,
		&vault.CreatedAt, &vault.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrVaultAlreadyExists
		}
		return nil, err
	}
	return &vault, nil
}

func (s *VaultService) GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (*models.Vault, error) {
	var vault models.Vault
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, salt, verification, created_at, updated_at
		FROM workspace_vaults
		WHERE workspace_id = $1
	`, workspaceID).Scan(
		&vault.ID, &vault.WorkspaceID, &vault.Salt, &vault.Verification,
		&vault.CreatedAt, &vault.UpdatedAt,
	)
	if err != nil {
		return nil, ErrVaultNotFound
	}
	return &vault, nil
}

func (s *VaultService) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		DELETE FROM workspace_vaults WHERE workspace_id = $1
	`, workspaceID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrVaultNotFound
	}
	return nil
}

func (s *VaultService) ListItems(ctx context.Context, vaultID uuid.UUID) ([]models.VaultItem, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, vault_id, data, created_at, updated_at
		FROM vault_items
		WHERE vault_id = $1
		ORDER BY created_at ASC
	`, vaultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.VaultItem
	for rows.Next() {
		var item models.VaultItem
		if err := rows.Scan(
			&item.ID, &item.VaultID, &item.Data,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *VaultService) CreateItem(ctx context.Context, vaultID uuid.UUID, data string) (*models.VaultItem, error) {
	var item models.VaultItem
	err := s.db.Pool.QueryRow(ctx, `
		INSERT INTO vault_items (vault_id, data)
		VALUES ($1, $2)
		RETURNING id, vault_id, data, created_at, updated_at
	`, vaultID, data).Scan(
		&item.ID, &item.VaultID, &item.Data,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *VaultService) UpdateItem(ctx context.Context, itemID, vaultID uuid.UUID, data string) (*models.VaultItem, error) {
	var item models.VaultItem
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE vault_items
		SET data = $1, updated_at = NOW()
		WHERE id = $2 AND vault_id = $3
		RETURNING id, vault_id, data, created_at, updated_at
	`, data, itemID, vaultID).Scan(
		&item.ID, &item.VaultID, &item.Data,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, ErrVaultItemNotFound
	}
	return &item, nil
}

func (s *VaultService) DeleteItem(ctx context.Context, itemID, vaultID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		DELETE FROM vault_items WHERE id = $1 AND vault_id = $2
	`, itemID, vaultID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrVaultItemNotFound
	}
	return nil
}
