# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## Project Overview

**sub2api** is an AI API gateway platform for subscription quota distribution. It generates API keys from upstream AI subscriptions (OpenAI, Antigravity, etc.) and distributes them to users with billing, load balancing, and request forwarding.

**Tech Stack**: Go 1.26.1 (Gin + Ent ORM) + Vue 3.4+ (Vite 5 + TailwindCSS + Pinia) + PostgreSQL 15+ + Redis 7+

## Repository Information

- **Upstream**: `Wei-Shaw/sub2api`
- **Fork**: `bayma888/sub2api-bmai`
- **Current branch**: `zhiguofan`

## Common Commands

### Backend (Go)

```bash
cd backend

# Run server
go run ./cmd/server/

# Build (with frontend embedded)
go build -tags embed -o sub2api ./cmd/server

# Unit tests
go test -tags=unit ./...

# Integration tests
go test -tags=integration ./...

# Lint (requires golangci-lint v2.9+)
golangci-lint run ./...

# Regenerate Ent ORM + Wire DI (after schema changes)
go generate ./ent
go generate ./cmd/server
```

### Frontend (Vue, pnpm)

```bash
cd frontend

# Install deps (MUST use pnpm, not npm)
pnpm install

# Dev server with hot reload
pnpm dev

# Type check + build
pnpm build

# Lint
pnpm run lint:check

# Type check
pnpm run typecheck

# Tests
pnpm test:run
```

### Local Development

```bash
# Start full local dev environment (backend + frontend)
./script/dev_local.sh up

# Check status
./script/dev_local.sh status

# View logs
./script/dev_local.sh logs

# Stop
./script/dev_local.sh down

# Custom ports
BACKEND_PORT=18082 FRONTEND_PORT=13002 ./script/dev_local.sh up
```

Default local dev ports:
- Backend: `http://127.0.0.1:8082`
- Frontend: `http://127.0.0.1:3002`
- PostgreSQL: `127.0.0.1:5432`
- Redis: `127.0.0.1:6379`

### Root Makefile

```bash
# Build both frontend and backend
make build

# Run all tests
make test

# Backend tests only
make test-backend

# Frontend lint + typecheck
make test-frontend
```

## Architecture

### Directory Structure

```
tokenpanel/
├── backend/
│   ├── cmd/server/           # Entry point, Wire DI setup
│   │   ├── main.go           # Application entry
│   │   ├── wire.go           # Wire DI definitions
│   │   └── wire_gen.go       # Generated DI code
│   ├── ent/                  # Ent ORM generated code
│   │   └── schema/           # Database schema definitions
│   ├── internal/
│   │   ├── handler/          # HTTP handlers (Gin)
│   │   │   ├── admin/        # Admin API handlers
│   │   │   └── dto/          # Request/Response DTOs
│   │   ├── service/          # Business logic layer
│   │   ├── repository/       # Data access layer
│   │   ├── server/           # HTTP server + route registration
│   │   │   ├── routes/       # Route definitions
│   │   │   └── middleware/   # HTTP middleware
│   │   ├── config/           # Configuration
│   │   ├── gateway/          # API gateway core (request routing)
│   │   └── web/              # Embedded frontend serving
│   └── migrations/           # Database migration scripts
├── frontend/
│   └── src/
│       ├── api/              # API client (Axios wrappers)
│       │   └── admin/        # Admin-specific API calls
│       ├── stores/           # Pinia state management
│       ├── views/            # Page components
│       │   ├── user/         # User-facing pages
│       │   └── admin/        # Admin dashboard pages
│       ├── components/       # Reusable Vue components
│       ├── router/index.ts   # Route definitions + auth guards
│       └── i18n/             # Internationalization
├── deploy/                   # Docker compose, install scripts
├── script/                   # Development scripts
│   ├── dev_local.sh          # Local dev environment
│   ├── sync_upstream_to_zhiguofan.sh
│   └── push_zhiguofan_to_internal_git.sh
└── claudedocs/               # Analysis documents
```

### Key Design Patterns

1. **Layered Architecture**: Handler → Service → Repository → Ent ORM
   - Handlers parse requests, call services, return responses
   - Services contain business logic
   - Repositories handle database queries via Ent
   - `golangci.yml` enforces: service must NOT import repository, handler must NOT import repository

2. **Dependency Injection**: Google Wire for DI, generated at `cmd/server/wire_gen.go`

3. **API Routes**: Defined in `internal/server/routes/`
   - Public routes: `/api/v1/auth/*`, `/api/v1/keys/*`
   - User routes: `/api/v1/dashboard`, `/api/v1/usage`
   - Admin routes: `/api/v1/admin/*`
   - Gateway routes: `/v1/messages`, `/v1/chat/completions` (forwarded to upstream AI APIs)

4. **Database**: Ent ORM with PostgreSQL. After any schema change in `ent/schema/`, run `go generate ./ent`.

5. **Frontend**: Vue 3 Composition API + Pinia stores + Vue Router
   - Auth guard in `router/index.ts` checks `requiresAuth` and `requiresAdmin` meta fields
   - API calls go through `frontend/src/api/` which wraps Axios

6. **Ent Schemas**: Key entities in `ent/schema/`:
   - `user.go` - Users
   - `account.go` - Upstream accounts (OpenAI, Antigravity, etc.)
   - `api_key.go` - User API keys
   - `group.go` - Account groups for routing
   - `usage_log.go` - Usage tracking

## Critical Development Rules

- **Frontend MUST use pnpm** (not npm). Submit `pnpm-lock.yaml` with every dependency change.
- **Go version must be 1.26.1** (CI enforces this)
- **Ent schema changes**: Always run `go generate ./ent` after editing schemas
- **Interface changes**: If you add methods to Go interfaces, ALL test stubs implementing that interface must be updated
- **Architecture enforcement**: Service layer must not import repository layer (enforced by golangci-lint)

## Branch Policy

- `main` branch tracks upstream `Wei-Shaw/sub2api`
- `zhiguofan` branch is the development branch for this fork
- Sync upstream: `./script/sync_upstream_to_zhiguofan.sh`
- Push to internal: `./script/push_zhiguofan_to_internal_git.sh`

## Model Mapping Pitfall

When batch-editing accounts across different platforms (OpenAI + Antigravity/Gemini), model whitelist/mappings can be overwritten by cross-platform policies. This causes OpenAI accounts to lose critical model mappings, resulting in "Service temporarily unavailable" errors.

**Fix**: Add pass-through mappings in batch edit, or recreate accounts. Avoid mixing platforms in batch operations.

## Testing Notes

- Unit tests: `go test -tags=unit ./...`
- Integration tests: `go test -tags=integration ./...` (requires PostgreSQL and Redis)
- Frontend tests: `pnpm test:run`
- E2E tests: `go test -tags=e2e -v -timeout=300s ./internal/integration/...`

## CI/CD

- **backend-ci.yml**: Runs on push/PR - unit tests, integration tests, golangci-lint
- **security-scan.yml**: Runs weekly - govulncheck, gosec, pnpm audit
- **release.yml**: Triggered on tag `v*` - builds and publishes releases