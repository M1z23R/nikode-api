package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
	"github.com/google/uuid"
)

var (
	ErrCannotRemoveOwner  = errors.New("cannot remove team owner")
	ErrMemberNotFound     = errors.New("member not found")
	ErrInviteNotFound     = errors.New("invite not found")
	ErrAlreadyMember      = errors.New("user is already a team member")
	ErrInviteAlreadyExists = errors.New("invite already exists")
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
	defer func() { _ = tx.Rollback(ctx) }()

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
	var role string
	err := s.db.Pool.QueryRow(ctx, `
		SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, userID).Scan(&role)
	if err != nil {
		return ErrMemberNotFound
	}

	if role == models.RoleOwner {
		return ErrCannotRemoveOwner
	}

	_, err = s.db.Pool.Exec(ctx, `
		DELETE FROM team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, userID)
	return err
}

func (s *TeamService) CreateInvite(ctx context.Context, teamID, inviterID, inviteeID uuid.UUID) (*models.TeamInvite, error) {
	isMember, err := s.IsMember(ctx, teamID, inviteeID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, ErrAlreadyMember
	}

	var invite models.TeamInvite
	err = s.db.Pool.QueryRow(ctx, `
		INSERT INTO team_invites (team_id, inviter_id, invitee_id, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (team_id, invitee_id) DO UPDATE SET
			inviter_id = EXCLUDED.inviter_id,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id, team_id, inviter_id, invitee_id, status, created_at, updated_at
	`, teamID, inviterID, inviteeID, models.InviteStatusPending).Scan(
		&invite.ID, &invite.TeamID, &invite.InviterID, &invite.InviteeID,
		&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create invite: %w", err)
	}
	return &invite, nil
}

func (s *TeamService) GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*models.TeamInvite, error) {
	var invite models.TeamInvite
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, team_id, inviter_id, invitee_id, status, created_at, updated_at
		FROM team_invites WHERE id = $1
	`, inviteID).Scan(
		&invite.ID, &invite.TeamID, &invite.InviterID, &invite.InviteeID,
		&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
	)
	if err != nil {
		return nil, ErrInviteNotFound
	}
	return &invite, nil
}

func (s *TeamService) GetUserPendingInvites(ctx context.Context, userID uuid.UUID) ([]models.TeamInvite, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT ti.id, ti.team_id, ti.inviter_id, ti.invitee_id, ti.status, ti.created_at, ti.updated_at,
		       t.id, t.name, t.owner_id, t.created_at, t.updated_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.created_at, u.updated_at
		FROM team_invites ti
		JOIN teams t ON ti.team_id = t.id
		JOIN users u ON ti.inviter_id = u.id
		WHERE ti.invitee_id = $1 AND ti.status = $2
		ORDER BY ti.created_at DESC
	`, userID, models.InviteStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []models.TeamInvite
	for rows.Next() {
		var invite models.TeamInvite
		var team models.Team
		var inviter models.User
		if err := rows.Scan(
			&invite.ID, &invite.TeamID, &invite.InviterID, &invite.InviteeID,
			&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
			&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt, &team.UpdatedAt,
			&inviter.ID, &inviter.Email, &inviter.Name, &inviter.AvatarURL,
			&inviter.Provider, &inviter.CreatedAt, &inviter.UpdatedAt,
		); err != nil {
			return nil, err
		}
		invite.Team = &team
		invite.Inviter = &inviter
		invites = append(invites, invite)
	}
	return invites, nil
}

func (s *TeamService) GetTeamPendingInvites(ctx context.Context, teamID uuid.UUID) ([]models.TeamInvite, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT ti.id, ti.team_id, ti.inviter_id, ti.invitee_id, ti.status, ti.created_at, ti.updated_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.created_at, u.updated_at
		FROM team_invites ti
		JOIN users u ON ti.invitee_id = u.id
		WHERE ti.team_id = $1 AND ti.status = $2
		ORDER BY ti.created_at DESC
	`, teamID, models.InviteStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []models.TeamInvite
	for rows.Next() {
		var invite models.TeamInvite
		var invitee models.User
		if err := rows.Scan(
			&invite.ID, &invite.TeamID, &invite.InviterID, &invite.InviteeID,
			&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
			&invitee.ID, &invitee.Email, &invitee.Name, &invitee.AvatarURL,
			&invitee.Provider, &invitee.CreatedAt, &invitee.UpdatedAt,
		); err != nil {
			return nil, err
		}
		invite.Invitee = &invitee
		invites = append(invites, invite)
	}
	return invites, nil
}

func (s *TeamService) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var invite models.TeamInvite
	err = tx.QueryRow(ctx, `
		SELECT id, team_id, invitee_id, status FROM team_invites WHERE id = $1
	`, inviteID).Scan(&invite.ID, &invite.TeamID, &invite.InviteeID, &invite.Status)
	if err != nil {
		return ErrInviteNotFound
	}

	if invite.InviteeID != userID {
		return ErrInviteNotFound
	}

	if invite.Status != models.InviteStatusPending {
		return ErrInviteNotFound
	}

	_, err = tx.Exec(ctx, `
		UPDATE team_invites SET status = $1, updated_at = NOW() WHERE id = $2
	`, models.InviteStatusAccepted, inviteID)
	if err != nil {
		return fmt.Errorf("failed to update invite: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO NOTHING
	`, invite.TeamID, userID, models.RoleMember)
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *TeamService) DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		UPDATE team_invites SET status = $1, updated_at = NOW()
		WHERE id = $2 AND invitee_id = $3 AND status = $4
	`, models.InviteStatusDeclined, inviteID, userID, models.InviteStatusPending)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrInviteNotFound
	}
	return nil
}

func (s *TeamService) CancelInvite(ctx context.Context, inviteID, teamID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		DELETE FROM team_invites WHERE id = $1 AND team_id = $2 AND status = $3
	`, inviteID, teamID, models.InviteStatusPending)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrInviteNotFound
	}
	return nil
}
