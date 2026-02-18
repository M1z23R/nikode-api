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
	ErrCannotRemoveOwner   = errors.New("cannot remove workspace owner")
	ErrMemberNotFound      = errors.New("member not found")
	ErrInviteNotFound      = errors.New("invite not found")
	ErrAlreadyMember       = errors.New("user is already a workspace member")
	ErrInviteAlreadyExists = errors.New("invite already exists")
)

type WorkspaceService struct {
	db *database.DB
}

func NewWorkspaceService(db *database.DB) *WorkspaceService {
	return &WorkspaceService{db: db}
}

func (s *WorkspaceService) Create(ctx context.Context, name string, ownerID uuid.UUID) (*models.Workspace, error) {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workspace models.Workspace
	err = tx.QueryRow(ctx, `
		INSERT INTO workspaces (name, owner_id)
		VALUES ($1, $2)
		RETURNING id, name, owner_id, created_at, updated_at
	`, name, ownerID).Scan(&workspace.ID, &workspace.Name, &workspace.OwnerID, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
	`, workspace.ID, ownerID, models.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to add owner as member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &workspace, nil
}

func (s *WorkspaceService) GetByID(ctx context.Context, workspaceID uuid.UUID) (*models.Workspace, error) {
	var workspace models.Workspace
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, name, owner_id, created_at, updated_at
		FROM workspaces WHERE id = $1
	`, workspaceID).Scan(&workspace.ID, &workspace.Name, &workspace.OwnerID, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (s *WorkspaceService) GetUserWorkspaces(ctx context.Context, userID uuid.UUID) ([]models.Workspace, []string, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT w.id, w.name, w.owner_id, w.created_at, w.updated_at, wm.role
		FROM workspaces w
		JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE wm.user_id = $1
		ORDER BY w.created_at DESC
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var workspaces []models.Workspace
	var roles []string
	for rows.Next() {
		var w models.Workspace
		var role string
		if err := rows.Scan(&w.ID, &w.Name, &w.OwnerID, &w.CreatedAt, &w.UpdatedAt, &role); err != nil {
			return nil, nil, err
		}
		workspaces = append(workspaces, w)
		roles = append(roles, role)
	}
	return workspaces, roles, nil
}

func (s *WorkspaceService) Update(ctx context.Context, workspaceID uuid.UUID, name string) (*models.Workspace, error) {
	var workspace models.Workspace
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE workspaces SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, name, owner_id, created_at, updated_at
	`, name, workspaceID).Scan(&workspace.ID, &workspace.Name, &workspace.OwnerID, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (s *WorkspaceService) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	return err
}

func (s *WorkspaceService) IsOwner(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	var ownerID uuid.UUID
	err := s.db.Pool.QueryRow(ctx, `SELECT owner_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

func (s *WorkspaceService) IsMember(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)
	`, workspaceID, userID).Scan(&exists)
	return exists, err
}

func (s *WorkspaceService) CanAccess(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	return s.IsMember(ctx, workspaceID, userID)
}

func (s *WorkspaceService) CanModify(ctx context.Context, workspaceID, userID uuid.UUID) (bool, error) {
	return s.IsOwner(ctx, workspaceID, userID)
}

func (s *WorkspaceService) GetMembers(ctx context.Context, workspaceID uuid.UUID) ([]models.WorkspaceMember, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT wm.id, wm.workspace_id, wm.user_id, wm.role, wm.created_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.created_at, u.updated_at
		FROM workspace_members wm
		JOIN users u ON wm.user_id = u.id
		WHERE wm.workspace_id = $1
		ORDER BY wm.created_at
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.WorkspaceMember
	for rows.Next() {
		var member models.WorkspaceMember
		var user models.User
		if err := rows.Scan(
			&member.ID, &member.WorkspaceID, &member.UserID, &member.Role, &member.CreatedAt,
			&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Provider, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		member.User = &user
		members = append(members, member)
	}
	return members, nil
}

func (s *WorkspaceService) AddMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
	`, workspaceID, userID, models.RoleMember)
	return err
}

func (s *WorkspaceService) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	var role string
	err := s.db.Pool.QueryRow(ctx, `
		SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, userID).Scan(&role)
	if err != nil {
		return ErrMemberNotFound
	}

	if role == models.RoleOwner {
		return ErrCannotRemoveOwner
	}

	_, err = s.db.Pool.Exec(ctx, `
		DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, userID)
	return err
}

func (s *WorkspaceService) CreateInvite(ctx context.Context, workspaceID, inviterID, inviteeID uuid.UUID) (*models.WorkspaceInvite, error) {
	isMember, err := s.IsMember(ctx, workspaceID, inviteeID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, ErrAlreadyMember
	}

	var invite models.WorkspaceInvite
	err = s.db.Pool.QueryRow(ctx, `
		INSERT INTO workspace_invites (workspace_id, inviter_id, invitee_id, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, invitee_id) DO UPDATE SET
			inviter_id = EXCLUDED.inviter_id,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id, workspace_id, inviter_id, invitee_id, status, created_at, updated_at
	`, workspaceID, inviterID, inviteeID, models.InviteStatusPending).Scan(
		&invite.ID, &invite.WorkspaceID, &invite.InviterID, &invite.InviteeID,
		&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create invite: %w", err)
	}
	return &invite, nil
}

func (s *WorkspaceService) GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*models.WorkspaceInvite, error) {
	var invite models.WorkspaceInvite
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, inviter_id, invitee_id, status, created_at, updated_at
		FROM workspace_invites WHERE id = $1
	`, inviteID).Scan(
		&invite.ID, &invite.WorkspaceID, &invite.InviterID, &invite.InviteeID,
		&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
	)
	if err != nil {
		return nil, ErrInviteNotFound
	}
	return &invite, nil
}

func (s *WorkspaceService) GetInviteWithDetails(ctx context.Context, inviteID uuid.UUID) (*models.WorkspaceInvite, error) {
	var invite models.WorkspaceInvite
	var workspace models.Workspace
	var inviter models.User
	var invitee models.User

	err := s.db.Pool.QueryRow(ctx, `
		SELECT wi.id, wi.workspace_id, wi.inviter_id, wi.invitee_id, wi.status, wi.created_at, wi.updated_at,
		       w.id, w.name, w.owner_id, w.created_at, w.updated_at,
		       inviter.id, inviter.email, inviter.name, inviter.avatar_url, inviter.provider, inviter.created_at, inviter.updated_at,
		       invitee.id, invitee.email, invitee.name, invitee.avatar_url, invitee.provider, invitee.created_at, invitee.updated_at
		FROM workspace_invites wi
		JOIN workspaces w ON wi.workspace_id = w.id
		JOIN users inviter ON wi.inviter_id = inviter.id
		JOIN users invitee ON wi.invitee_id = invitee.id
		WHERE wi.id = $1
	`, inviteID).Scan(
		&invite.ID, &invite.WorkspaceID, &invite.InviterID, &invite.InviteeID,
		&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
		&workspace.ID, &workspace.Name, &workspace.OwnerID, &workspace.CreatedAt, &workspace.UpdatedAt,
		&inviter.ID, &inviter.Email, &inviter.Name, &inviter.AvatarURL,
		&inviter.Provider, &inviter.CreatedAt, &inviter.UpdatedAt,
		&invitee.ID, &invitee.Email, &invitee.Name, &invitee.AvatarURL,
		&invitee.Provider, &invitee.CreatedAt, &invitee.UpdatedAt,
	)
	if err != nil {
		return nil, ErrInviteNotFound
	}

	invite.Workspace = &workspace
	invite.Inviter = &inviter
	invite.Invitee = &invitee
	return &invite, nil
}

func (s *WorkspaceService) GetUserPendingInvites(ctx context.Context, userID uuid.UUID) ([]models.WorkspaceInvite, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT wi.id, wi.workspace_id, wi.inviter_id, wi.invitee_id, wi.status, wi.created_at, wi.updated_at,
		       w.id, w.name, w.owner_id, w.created_at, w.updated_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.created_at, u.updated_at
		FROM workspace_invites wi
		JOIN workspaces w ON wi.workspace_id = w.id
		JOIN users u ON wi.inviter_id = u.id
		WHERE wi.invitee_id = $1 AND wi.status = $2
		ORDER BY wi.created_at DESC
	`, userID, models.InviteStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []models.WorkspaceInvite
	for rows.Next() {
		var invite models.WorkspaceInvite
		var workspace models.Workspace
		var inviter models.User
		if err := rows.Scan(
			&invite.ID, &invite.WorkspaceID, &invite.InviterID, &invite.InviteeID,
			&invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
			&workspace.ID, &workspace.Name, &workspace.OwnerID, &workspace.CreatedAt, &workspace.UpdatedAt,
			&inviter.ID, &inviter.Email, &inviter.Name, &inviter.AvatarURL,
			&inviter.Provider, &inviter.CreatedAt, &inviter.UpdatedAt,
		); err != nil {
			return nil, err
		}
		invite.Workspace = &workspace
		invite.Inviter = &inviter
		invites = append(invites, invite)
	}
	return invites, nil
}

func (s *WorkspaceService) GetWorkspacePendingInvites(ctx context.Context, workspaceID uuid.UUID) ([]models.WorkspaceInvite, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT wi.id, wi.workspace_id, wi.inviter_id, wi.invitee_id, wi.status, wi.created_at, wi.updated_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.created_at, u.updated_at
		FROM workspace_invites wi
		JOIN users u ON wi.invitee_id = u.id
		WHERE wi.workspace_id = $1 AND wi.status = $2
		ORDER BY wi.created_at DESC
	`, workspaceID, models.InviteStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []models.WorkspaceInvite
	for rows.Next() {
		var invite models.WorkspaceInvite
		var invitee models.User
		if err := rows.Scan(
			&invite.ID, &invite.WorkspaceID, &invite.InviterID, &invite.InviteeID,
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

func (s *WorkspaceService) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var invite models.WorkspaceInvite
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, invitee_id, status FROM workspace_invites WHERE id = $1
	`, inviteID).Scan(&invite.ID, &invite.WorkspaceID, &invite.InviteeID, &invite.Status)
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
		UPDATE workspace_invites SET status = $1, updated_at = NOW() WHERE id = $2
	`, models.InviteStatusAccepted, inviteID)
	if err != nil {
		return fmt.Errorf("failed to update invite: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
	`, invite.WorkspaceID, userID, models.RoleMember)
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *WorkspaceService) DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		UPDATE workspace_invites SET status = $1, updated_at = NOW()
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

func (s *WorkspaceService) CancelInvite(ctx context.Context, inviteID, workspaceID uuid.UUID) error {
	result, err := s.db.Pool.Exec(ctx, `
		DELETE FROM workspace_invites WHERE id = $1 AND workspace_id = $2 AND status = $3
	`, inviteID, workspaceID, models.InviteStatusPending)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrInviteNotFound
	}
	return nil
}
