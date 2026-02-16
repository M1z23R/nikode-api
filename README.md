# Nikode API

The official backend server for [Nikode](https://github.com/M1z23R/nikode), an Electron-based API client similar to Postman or Bruno. This Go REST API enables team collaboration features including users, teams, workspaces, and collections with real-time synchronization via Server-Sent Events (SSE).

If you want to self-host your own Nikode backend for your team or organization, this repository provides everything you need.

## Features

- **OAuth Authentication**: Support for GitHub, GitLab, and Google providers
- **JWT-based Authorization**: Secure access and refresh token system with automatic rotation
- **Teams**: Create teams, invite members by email, manage roles (owner/member)
- **Workspaces**: Personal workspaces for individual users or team workspaces for collaboration
- **Collections**: Store API collection data as JSON with optimistic locking for conflict resolution
- **Real-time Sync**: Server-Sent Events for live updates when collections change

## Requirements

- Go 1.21+
- PostgreSQL 14+
- OAuth application credentials from at least one provider (GitHub, GitLab, or Google)

## Configuration

Create a `.env` file in the project root or set environment variables:

```env
# Server
PORT=8080
ENV=production

# Database
DATABASE_URL=postgres://user:password@localhost:5432/nikode?sslmode=disable

# JWT
JWT_SECRET=your-secure-random-secret-here
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Frontend callback (Electron deep link)
FRONTEND_CALLBACK_URL=nikode://auth/callback

# GitHub OAuth (optional)
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret
GITHUB_REDIRECT_URL=https://your-domain.com/api/v1/auth/github/callback

# GitLab OAuth (optional)
GITLAB_CLIENT_ID=your-gitlab-client-id
GITLAB_CLIENT_SECRET=your-gitlab-client-secret
GITLAB_REDIRECT_URL=https://your-domain.com/api/v1/auth/gitlab/callback

# Google OAuth (optional)
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URL=https://your-domain.com/api/v1/auth/google/callback
```

OAuth providers are dynamically enabled based on whether their client ID is configured.

## Database Setup

The application runs migrations automatically on startup. Ensure your PostgreSQL database exists and the connection URL is correct.

```bash
createdb nikode
```

## Installation

### Development

```bash
# Run directly
make dev

# Or build and run
make run
```

### Production (systemd)

The included systemd service file provides a secure deployment setup:

```bash
# First-time setup: creates nikode user, installs service, copies binary
sudo make setup

# Subsequent deployments
make deploy
```

The service runs as a dedicated `nikode` user with restricted permissions and restarts automatically on failure.

## API Overview

All endpoints are under `/api/v1`. See [API.md](API.md) for complete documentation.

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/auth/:provider/consent` | Get OAuth consent URL |
| GET | `/auth/:provider/callback` | OAuth callback (redirects to Electron app) |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/logout` | Revoke refresh token |
| POST | `/auth/logout-all` | Revoke all refresh tokens (protected) |

### Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/users/me` | Get current user profile |
| PATCH | `/users/me` | Update display name |

### Teams

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/teams` | List user's teams |
| POST | `/teams` | Create team |
| GET | `/teams/:id` | Get team details |
| PATCH | `/teams/:id` | Rename team (owner) |
| DELETE | `/teams/:id` | Delete team (owner) |
| GET | `/teams/:id/members` | List team members |
| POST | `/teams/:id/members` | Invite member by email (owner) |
| DELETE | `/teams/:id/members/:memberId` | Remove member (owner) |
| POST | `/teams/:id/leave` | Leave team (members) |

### Workspaces

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/workspaces` | List accessible workspaces |
| POST | `/workspaces` | Create workspace |
| GET | `/workspaces/:id` | Get workspace details |
| PATCH | `/workspaces/:id` | Rename workspace (owner) |
| DELETE | `/workspaces/:id` | Delete workspace (owner) |

### Collections

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/workspaces/:id/collections` | List collections |
| POST | `/workspaces/:id/collections` | Create collection |
| GET | `/workspaces/:id/collections/:collectionId` | Get collection |
| PATCH | `/workspaces/:id/collections/:collectionId` | Update collection (with version) |
| DELETE | `/workspaces/:id/collections/:collectionId` | Delete collection (owner) |

### Real-time Events (SSE)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/workspaces/:id/events` | Open SSE stream |
| POST | `/sse/:clientId/subscribe/:workspaceId` | Subscribe to workspace |
| POST | `/sse/:clientId/unsubscribe/:workspaceId` | Unsubscribe from workspace |

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |

## Architecture

```
nikode-api/
├── cmd/nikode-api/        # Application entry point
├── internal/
│   ├── config/            # Environment configuration
│   ├── database/          # PostgreSQL connection and migrations
│   ├── handlers/          # HTTP request handlers
│   ├── middleware/        # JWT authentication middleware
│   ├── models/            # Domain models
│   ├── oauth/             # OAuth provider implementations
│   ├── services/          # Business logic
│   └── sse/               # Server-Sent Events hub
└── pkg/dto/               # Request/response DTOs
```

## Security

- JWT tokens are signed with HS256
- Refresh tokens are stored as SHA-256 hashes (not plain text)
- OAuth state parameters are cryptographically random and single-use
- Expired tokens are automatically cleaned up
- The systemd service runs with restricted privileges

## Optimistic Locking

Collections use version-based optimistic locking to handle concurrent edits:

1. Client fetches a collection and receives its `version`
2. Client sends an update with the expected `version`
3. If the version matches, the update succeeds and version increments
4. If the version doesn't match (another client updated first), a `409 VERSION_CONFLICT` error is returned with the current version
5. Client can then fetch the latest data and retry

## License

MIT
