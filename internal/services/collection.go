package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
)

var (
	ErrVersionConflict  = errors.New("version conflict: collection has been modified")
	ErrCollectionNotFound = errors.New("collection not found")
	ErrNoFieldsToUpdate = errors.New("no fields to update")
)

type CollectionService struct {
	db *database.DB
}

func NewCollectionService(db *database.DB) *CollectionService {
	return &CollectionService{db: db}
}

func (s *CollectionService) Create(ctx context.Context, workspaceID uuid.UUID, name string, data json.RawMessage, userID uuid.UUID) (*models.Collection, error) {
	if data == nil {
		data = json.RawMessage("{}")
	}

	var collection models.Collection
	err := s.db.Pool.QueryRow(ctx, `
		INSERT INTO collections (workspace_id, name, data, updated_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, name, data, version, updated_by, created_at, updated_at
	`, workspaceID, name, data, userID).Scan(
		&collection.ID, &collection.WorkspaceID, &collection.Name,
		&collection.Data, &collection.Version, &collection.UpdatedBy,
		&collection.CreatedAt, &collection.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &collection, nil
}

func (s *CollectionService) GetByID(ctx context.Context, collectionID uuid.UUID) (*models.Collection, error) {
	var collection models.Collection
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, name, data, version, updated_by, created_at, updated_at
		FROM collections WHERE id = $1
	`, collectionID).Scan(
		&collection.ID, &collection.WorkspaceID, &collection.Name,
		&collection.Data, &collection.Version, &collection.UpdatedBy,
		&collection.CreatedAt, &collection.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &collection, nil
}

func (s *CollectionService) GetByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]models.Collection, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, workspace_id, name, data, version, updated_by, created_at, updated_at
		FROM collections WHERE workspace_id = $1
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []models.Collection
	for rows.Next() {
		var c models.Collection
		if err := rows.Scan(
			&c.ID, &c.WorkspaceID, &c.Name, &c.Data, &c.Version,
			&c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, nil
}

func (s *CollectionService) Update(ctx context.Context, collectionID uuid.UUID, name *string, data json.RawMessage, expectedVersion int, userID uuid.UUID) (*models.Collection, error) {
	var collection models.Collection

	if name != nil && data != nil {
		err := s.db.Pool.QueryRow(ctx, `
			UPDATE collections
			SET name = $1, data = $2, version = version + 1, updated_by = $3, updated_at = NOW()
			WHERE id = $4 AND version = $5
			RETURNING id, workspace_id, name, data, version, updated_by, created_at, updated_at
		`, *name, data, userID, collectionID, expectedVersion).Scan(
			&collection.ID, &collection.WorkspaceID, &collection.Name,
			&collection.Data, &collection.Version, &collection.UpdatedBy,
			&collection.CreatedAt, &collection.UpdatedAt,
		)
		if err != nil {
			return nil, s.checkVersionConflict(ctx, collectionID, expectedVersion, err)
		}
	} else if name != nil {
		err := s.db.Pool.QueryRow(ctx, `
			UPDATE collections
			SET name = $1, version = version + 1, updated_by = $2, updated_at = NOW()
			WHERE id = $3 AND version = $4
			RETURNING id, workspace_id, name, data, version, updated_by, created_at, updated_at
		`, *name, userID, collectionID, expectedVersion).Scan(
			&collection.ID, &collection.WorkspaceID, &collection.Name,
			&collection.Data, &collection.Version, &collection.UpdatedBy,
			&collection.CreatedAt, &collection.UpdatedAt,
		)
		if err != nil {
			return nil, s.checkVersionConflict(ctx, collectionID, expectedVersion, err)
		}
	} else if data != nil {
		err := s.db.Pool.QueryRow(ctx, `
			UPDATE collections
			SET data = $1, version = version + 1, updated_by = $2, updated_at = NOW()
			WHERE id = $3 AND version = $4
			RETURNING id, workspace_id, name, data, version, updated_by, created_at, updated_at
		`, data, userID, collectionID, expectedVersion).Scan(
			&collection.ID, &collection.WorkspaceID, &collection.Name,
			&collection.Data, &collection.Version, &collection.UpdatedBy,
			&collection.CreatedAt, &collection.UpdatedAt,
		)
		if err != nil {
			return nil, s.checkVersionConflict(ctx, collectionID, expectedVersion, err)
		}
	} else {
		return nil, ErrNoFieldsToUpdate
	}

	return &collection, nil
}

func (s *CollectionService) checkVersionConflict(ctx context.Context, collectionID uuid.UUID, expectedVersion int, originalErr error) error {
	var currentVersion int
	err := s.db.Pool.QueryRow(ctx, `SELECT version FROM collections WHERE id = $1`, collectionID).Scan(&currentVersion)
	if err != nil {
		return ErrCollectionNotFound
	}
	if currentVersion != expectedVersion {
		return ErrVersionConflict
	}
	return originalErr
}

func (s *CollectionService) Delete(ctx context.Context, collectionID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM collections WHERE id = $1`, collectionID)
	return err
}
