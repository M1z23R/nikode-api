package services

import (
	"context"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
)

type TemplateService struct {
	db *database.DB
}

func NewTemplateService(db *database.DB) *TemplateService {
	return &TemplateService{db: db}
}

func (s *TemplateService) Search(ctx context.Context, query string, limit int) ([]models.PublicTemplate, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, name, description, category, data, created_at, updated_at
		FROM public_templates
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name ASC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []models.PublicTemplate
	for rows.Next() {
		var t models.PublicTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Data, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (s *TemplateService) GetByID(ctx context.Context, id uuid.UUID) (*models.PublicTemplate, error) {
	var t models.PublicTemplate
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, name, description, category, data, created_at, updated_at
		FROM public_templates
		WHERE id = $1
	`, id).Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Data, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *TemplateService) Create(ctx context.Context, name, description, category string, data []byte) (*models.PublicTemplate, error) {
	var t models.PublicTemplate
	err := s.db.Pool.QueryRow(ctx, `
		INSERT INTO public_templates (name, description, category, data)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, category, data, created_at, updated_at
	`, name, description, category, data).Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Data, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM public_templates WHERE id = $1`, id)
	return err
}
