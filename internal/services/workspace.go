package services

import (
	"context"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
)

type WorkspaceService struct {
	db          *database.DB
	teamService *TeamService
}

func NewWorkspaceService(db *database.DB, teamService *TeamService) *WorkspaceService {
	return &WorkspaceService{db: db, teamService: teamService}
}

func (s *WorkspaceService) Create(ctx context.Context, name string, userID uuid.UUID, teamID *uuid.UUID) (*models.Workspace, error) {
	var workspace models.Workspace
	var err error

	if teamID != nil {
		err = s.db.Pool.QueryRow(ctx, `
			INSERT INTO workspaces (name, team_id)
			VALUES ($1, $2)
			RETURNING id, name, user_id, team_id, created_at, updated_at
		`, name, teamID).Scan(&workspace.ID, &workspace.Name, &workspace.UserID, &workspace.TeamID, &workspace.CreatedAt, &workspace.UpdatedAt)
	} else {
		err = s.db.Pool.QueryRow(ctx, `
			INSERT INTO workspaces (name, user_id)
			VALUES ($1, $2)
			RETURNING id, name, user_id, team_id, created_at, updated_at
		`, name, userID).Scan(&workspace.ID, &workspace.Name, &workspace.UserID, &workspace.TeamID, &workspace.CreatedAt, &workspace.UpdatedAt)
	}

	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (s *WorkspaceService) GetByID(ctx context.Context, workspaceID uuid.UUID) (*models.Workspace, error) {
	var workspace models.Workspace
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, name, user_id, team_id, created_at, updated_at
		FROM workspaces WHERE id = $1
	`, workspaceID).Scan(&workspace.ID, &workspace.Name, &workspace.UserID, &workspace.TeamID, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (s *WorkspaceService) GetUserWorkspaces(ctx context.Context, userID uuid.UUID) ([]models.Workspace, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT DISTINCT w.id, w.name, w.user_id, w.team_id, w.created_at, w.updated_at
		FROM workspaces w
		LEFT JOIN team_members tm ON w.team_id = tm.team_id
		WHERE w.user_id = $1 OR tm.user_id = $1
		ORDER BY w.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []models.Workspace
	for rows.Next() {
		var w models.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.UserID, &w.TeamID, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, nil
}

func (s *WorkspaceService) Update(ctx context.Context, workspaceID uuid.UUID, name string) (*models.Workspace, error) {
	var workspace models.Workspace
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE workspaces SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, name, user_id, team_id, created_at, updated_at
	`, name, workspaceID).Scan(&workspace.ID, &workspace.Name, &workspace.UserID, &workspace.TeamID, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (s *WorkspaceService) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	return err
}

func (s *WorkspaceService) CanAccess(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	workspace, err := s.GetByID(ctx, workspaceID)
	if err != nil {
		return false, err
	}

	if workspace.UserID != nil && *workspace.UserID == userID {
		return true, nil
	}

	if workspace.TeamID != nil {
		return s.teamService.IsMember(ctx, *workspace.TeamID, userID)
	}

	return false, nil
}

func (s *WorkspaceService) CanModify(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	workspace, err := s.GetByID(ctx, workspaceID)
	if err != nil {
		return false, err
	}

	if workspace.UserID != nil && *workspace.UserID == userID {
		return true, nil
	}

	if workspace.TeamID != nil {
		return s.teamService.IsOwner(ctx, *workspace.TeamID, userID)
	}

	return false, nil
}
