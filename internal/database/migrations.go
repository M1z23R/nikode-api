package database

import (
	"context"
	"fmt"
)

var migrations = []string{
	`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

	`CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		email VARCHAR(255) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		avatar_url VARCHAR(500),
		provider VARCHAR(50) NOT NULL,
		provider_id VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(provider, provider_id)
	)`,

	`CREATE TABLE IF NOT EXISTS workspaces (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name VARCHAR(255) NOT NULL,
		owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	// Ensure owner_id exists for databases created with older schema versions
	// This MUST come immediately after CREATE TABLE workspaces to prevent failures
	// in subsequent migrations that reference owner_id
	`ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE CASCADE`,

	`CREATE TABLE IF NOT EXISTS workspace_members (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		role VARCHAR(50) NOT NULL DEFAULT 'member',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(workspace_id, user_id)
	)`,

	`CREATE TABLE IF NOT EXISTS workspace_invites (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		invitee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(workspace_id, invitee_id)
	)`,

	`CREATE TABLE IF NOT EXISTS collections (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		data JSONB NOT NULL DEFAULT '{}',
		version INTEGER NOT NULL DEFAULT 1,
		updated_by UUID REFERENCES users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS refresh_tokens (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash VARCHAR(255) NOT NULL UNIQUE,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE INDEX IF NOT EXISTS idx_workspace_members_workspace_id ON workspace_members(workspace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_workspace_members_user_id ON workspace_members(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_workspace_invites_workspace_id ON workspace_invites(workspace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_workspace_invites_invitee_id ON workspace_invites(invitee_id)`,
	`CREATE INDEX IF NOT EXISTS idx_collections_workspace_id ON collections(workspace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id)`,

	// Migration: Index owner_id
	`CREATE INDEX IF NOT EXISTS idx_workspaces_owner_id ON workspaces(owner_id)`,

	// Migration: Populate owner_id from user_id for personal workspaces
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'workspaces' AND column_name = 'user_id'
		) THEN
			UPDATE workspaces SET owner_id = user_id WHERE owner_id IS NULL AND user_id IS NOT NULL;
		END IF;
	END $$`,

	// Migration: Populate owner_id from team owner for team workspaces
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'teams'
		) THEN
			UPDATE workspaces w SET owner_id = t.owner_id
			FROM teams t
			WHERE w.owner_id IS NULL AND w.team_id IS NOT NULL AND w.team_id = t.id;
		END IF;
	END $$`,

	// Migration: Create workspace_members for personal workspaces (owner as member)
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'workspaces' AND column_name = 'user_id'
		) THEN
			INSERT INTO workspace_members (workspace_id, user_id, role)
			SELECT w.id, w.user_id, 'owner'
			FROM workspaces w
			WHERE w.user_id IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM workspace_members wm WHERE wm.workspace_id = w.id AND wm.user_id = w.user_id);
		END IF;
	END $$`,

	// Migration: Create workspace_members for team workspaces from team_members
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'team_members'
		) THEN
			INSERT INTO workspace_members (workspace_id, user_id, role)
			SELECT w.id, tm.user_id, tm.role
			FROM workspaces w
			JOIN team_members tm ON w.team_id = tm.team_id
			WHERE w.team_id IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM workspace_members wm WHERE wm.workspace_id = w.id AND wm.user_id = tm.user_id);
		END IF;
	END $$`,

	// Migration: Drop old columns from workspaces table
	`ALTER TABLE workspaces DROP COLUMN IF EXISTS user_id`,
	`ALTER TABLE workspaces DROP COLUMN IF EXISTS team_id`,

	// Migration: Drop old team tables (order matters due to FK constraints)
	`DROP TABLE IF EXISTS team_invites`,
	`DROP TABLE IF EXISTS team_members`,
	`DROP TABLE IF EXISTS teams`,

	// Migration: Drop old indexes
	`DROP INDEX IF EXISTS idx_workspaces_user_id`,
	`DROP INDEX IF EXISTS idx_workspaces_team_id`,
	`DROP INDEX IF EXISTS idx_team_members_team_id`,
	`DROP INDEX IF EXISTS idx_team_members_user_id`,
	`DROP INDEX IF EXISTS idx_team_invites_team_id`,
	`DROP INDEX IF EXISTS idx_team_invites_invitee_id`,

	// Migration: Make owner_id NOT NULL after migration (skip if already not null)
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'workspaces' AND column_name = 'owner_id' AND is_nullable = 'YES'
		) THEN
			-- Delete any workspaces with null owner_id (shouldn't exist, but safety)
			DELETE FROM workspaces WHERE owner_id IS NULL;
			-- Make the column NOT NULL
			ALTER TABLE workspaces ALTER COLUMN owner_id SET NOT NULL;
		END IF;
	END $$`,

	// Workspace API Keys for CI/CD automation
	`CREATE TABLE IF NOT EXISTS workspace_api_keys (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		key_hash VARCHAR(255) NOT NULL UNIQUE,
		key_prefix VARCHAR(20) NOT NULL,
		created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at TIMESTAMP WITH TIME ZONE,
		revoked_at TIMESTAMP WITH TIME ZONE,
		last_used_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_workspace_api_keys_workspace_id ON workspace_api_keys(workspace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_workspace_api_keys_key_hash ON workspace_api_keys(key_hash)`,

	// Workspace Vaults (zero-knowledge encrypted vault per workspace)
	`CREATE TABLE IF NOT EXISTS workspace_vaults (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE UNIQUE,
		salt TEXT NOT NULL,
		verification TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS vault_items (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		vault_id UUID NOT NULL REFERENCES workspace_vaults(id) ON DELETE CASCADE,
		data TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE INDEX IF NOT EXISTS idx_vault_items_vault_id ON vault_items(vault_id)`,

	// Public templates for collection creation
	`CREATE EXTENSION IF NOT EXISTS pg_trgm`,

	`CREATE TABLE IF NOT EXISTS public_templates (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name VARCHAR(255) NOT NULL,
		description TEXT,
		category VARCHAR(100),
		data JSONB NOT NULL DEFAULT '{}',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE INDEX IF NOT EXISTS idx_public_templates_name_search ON public_templates USING gin (name gin_trgm_ops)`,

	// Migration: Add global_role column to users table
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS global_role VARCHAR(50) NOT NULL DEFAULT 'user'`,
}

func (db *DB) Migrate(ctx context.Context) error {
	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}
