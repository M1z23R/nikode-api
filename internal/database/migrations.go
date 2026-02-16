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

	`CREATE TABLE IF NOT EXISTS teams (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name VARCHAR(255) NOT NULL,
		owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS team_members (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		role VARCHAR(50) NOT NULL DEFAULT 'member',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(team_id, user_id)
	)`,

	`CREATE TABLE IF NOT EXISTS workspaces (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name VARCHAR(255) NOT NULL,
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		CONSTRAINT workspace_owner_check CHECK (
			(user_id IS NOT NULL AND team_id IS NULL) OR
			(user_id IS NULL AND team_id IS NOT NULL)
		)
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

	`CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_workspaces_user_id ON workspaces(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_workspaces_team_id ON workspaces(team_id)`,
	`CREATE INDEX IF NOT EXISTS idx_collections_workspace_id ON collections(workspace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id)`,
}

func (db *DB) Migrate(ctx context.Context) error {
	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}
