package services

import (
	"context"
	"fmt"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
)

type TeamService struct {
	db *database.DB
}

func NewTeamService(db *database.DB) *TeamService {
	return &TeamService{db: db}
}

func (s *TeamService) Create(ctx context.Context, name string, ownerID uuid.UUID) (*models.Team, error) {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var team models.Team
	err = tx.QueryRow(ctx, `
		INSERT INTO teams (name, owner_id)
		VALUES ($1, $2)
		RETURNING id, name, owner_id, created_at, updated_at
	`, name, ownerID).Scan(&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
	`, team.ID, ownerID, models.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to add owner as member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &team, nil
}

func (s *TeamService) GetByID(ctx context.Context, teamID uuid.UUID) (*models.Team, error) {
	var team models.Team
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, name, owner_id, created_at, updated_at
		FROM teams WHERE id = $1
	`, teamID).Scan(&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *TeamService) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, []string, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT t.id, t.name, t.owner_id, t.created_at, t.updated_at, tm.role
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.created_at DESC
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var teams []models.Team
	var roles []string
	for rows.Next() {
		var team models.Team
		var role string
		if err := rows.Scan(&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt, &team.UpdatedAt, &role); err != nil {
			return nil, nil, err
		}
		teams = append(teams, team)
		roles = append(roles, role)
	}
	return teams, roles, nil
}

func (s *TeamService) Update(ctx context.Context, teamID uuid.UUID, name string) (*models.Team, error) {
	var team models.Team
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE teams SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, name, owner_id, created_at, updated_at
	`, name, teamID).Scan(&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *TeamService) Delete(ctx context.Context, teamID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, teamID)
	return err
}

func (s *TeamService) IsOwner(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	var ownerID uuid.UUID
	err := s.db.Pool.QueryRow(ctx, `SELECT owner_id FROM teams WHERE id = $1`, teamID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

func (s *TeamService) IsMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)
	`, teamID, userID).Scan(&exists)
	return exists, err
}

func (s *TeamService) GetMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMember, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.created_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.created_at, u.updated_at
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1
		ORDER BY tm.created_at
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.TeamMember
	for rows.Next() {
		var member models.TeamMember
		var user models.User
		if err := rows.Scan(
			&member.ID, &member.TeamID, &member.UserID, &member.Role, &member.CreatedAt,
			&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Provider, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		member.User = &user
		members = append(members, member)
	}
	return members, nil
}

func (s *TeamService) AddMember(ctx context.Context, teamID, userID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO NOTHING
	`, teamID, userID, models.RoleMember)
	return err
}

func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `
		DELETE FROM team_members WHERE team_id = $1 AND user_id = $2 AND role != $3
	`, teamID, userID, models.RoleOwner)
	return err
}
