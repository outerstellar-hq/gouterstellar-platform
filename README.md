# gouterstellar-platform

Go port of the [outerstellar-platform](https://github.com/outerstellar-hq/outerstellar-platform) web server — a single static binary that serves the same HTTP API for the Java Swing desktop client, with ~10-15x lower RAM usage and ~50x faster cold start.

## Quick Start

```bash
# Prerequisites: Go 1.24+, PostgreSQL 16+, Podman or Docker
make build          # build the server
make migrate-up     # apply database migrations
make seed           # create initial admin user (admin / admin123)
make dev            # run with dev profile (CSRF off, dev dashboard on)
```

The server listens on `http://localhost:8080` by default.

Authentication includes BCrypt password hashing, durable account lockout, optional authenticator-app TOTP, one-time backup codes, sessions, API keys, JWT, and Google OAuth.

## Configuration

Config is loaded from `config/application.yaml` with optional profile overrides (`config/application-dev.yaml`). All values can be overridden with environment variables.

```bash
APP_PROFILE=dev      # activate the dev profile
DATABASE_URL=...     # override the database connection string
PORT=8080            # override the listen port
```

Key settings:

| Key | Default | Description |
|-----|---------|-------------|
| `port` | `8080` | Listen port |
| `database_url` | `postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable` | PostgreSQL connection string |
| `dev_mode` | `false` | Enable development mode |
| `csrf_enabled` | `true` | Enable CSRF protection |
| `session_cookie_secure` | `false` | Set Secure flag on session cookies |
| `session_timeout_minutes` | `30` | Session timeout |
| `max_failed_login_attempts` | `10` | Failed logins before a timed account lockout |
| `lockout_duration_seconds` | `900` | Account lockout duration |
| `jwt.enabled` | `false` | Enable JWT token auth |
| `email.enabled` | `false` | Enable email sending |

## Project Structure

```
gouterstellar-platform/
├── cmd/
│   ├── server/main.go          # Server entry point
│   ├── seed/main.go            # Admin user seeder
│   └── migrate/main.go         # Database migration runner
├── config/
│   ├── application.yaml        # Base configuration
│   └── application-dev.yaml    # Dev profile overrides
├── queries/                    # sqlc query definitions
├── internal/
│   ├── model/                  # Domain models
│   ├── config/                 # YAML and environment configuration loading
│   ├── persistence/            # Repository implementations + sqlc generated code
│   ├── security/               # Auth, bcrypt, TOTP, JWT, permissions, API keys, OAuth
│   ├── platform/core/          # Core extension and versioned SQL migrations
│   ├── service/                # Business logic (message, contact, security, email, outbox)
│   ├── web/
│   │   ├── filter/             # Chi middleware (CORS, CSRF, auth, rate limiting, metrics)
│   │   ├── handler/            # HTTP handlers (sync API, auth, contacts, admin)
│   │   ├── template/           # Go html/template files
│   │   └── viewmodel/          # Template view models
│   └── wire/                   # Manual dependency injection
├── pkg/
│   └── i18n/                   # Internationalization (locales, .properties, placeholder injection)
├── static/
│   ├── css/main.css            # Stylesheet with light/dark theming
│   └── js/platform.js          # Theme toggle, CSRF injection, toast auto-dismiss
├── sqlc.yaml                   # sqlc code generation config
└── .golangci-lint.yml          # Linter configuration
```

## Required Tools

Install these before contributing:

### Build & Run

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest    # SQL code generation
```

### Linting & Static Analysis

All of these must pass before committing. `make check` runs the first three together.

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest   # Meta-linter (50+ linters)
go install mvdan.cc/gofumpt@latest                                       # Stricter gofmt
go install golang.org/x/tools/cmd/goimports@latest                       # Import ordering
go install github.com/securego/gosec/v2/cmd/gosec@latest                 # Security scanner
```

| Tool | Command | What it catches |
|------|---------|-----------------|
| `golangci-lint` | `make lint` | All categories below combined |
| `gofumpt` | `make fmt` | Formatting (stricter than gofmt) |
| `goimports` | `make fmt` | Import ordering, unused imports |
| `go vet` | `make vet` | Common mistakes (printf args, unreachable code, locks) |
| `staticcheck` | via golangci-lint | Unused code, simplifications, deprecations |
| `gosec` | `make security` | SQL injection, hardcoded credentials, weak crypto |
| `gocyclo` | via golangci-lint | Cyclomatic complexity > 15 |
| `dupl` | via golangci-lint | Duplicate code blocks |
| `errcheck` | via golangci-lint | Unchecked error returns |
| `misspell` | via golangci-lint | Typos in comments/strings |
| `revive` | via golangci-lint | Style, naming, exported docs |

### Running Checks

```bash
make fmt          # auto-format all Go files (gofumpt + goimports)
make vet          # run go vet
make lint         # run golangci-lint with project config
make security     # run gosec security scanner
make check        # fmt + vet + lint in sequence
make lint-full    # run golangci-lint with ALL linters enabled
```

### Configuration

The linter config is in `.golangci-lint.yml`. Key settings:

- **Complexity**: functions must be under 80 lines, cyclomatic complexity under 15
- **Errors**: all error returns must be checked (`errcheck` with type assertions)
- **Formatting**: `gofumpt` extra rules, `goimports` with local prefix grouping
- **Tests**: relaxed rules for `_test.go` files (dupl, gosec, funlen, gocyclo exempted)
- **Generated code**: `internal/persistence/db/` is fully excluded (sqlc output)

## Testing

```bash
make test                           # run all tests
go test ./internal/service/ -v      # run service layer tests with verbose output
go test -run TestAuthenticate ./... # run a specific test
```

## Database

```bash
make migrate-up    # apply pending migrations
make seed          # create admin user (pass -username / -password flags)
```

## API Endpoints

### Sync API (for desktop client)
- `GET /api/v1/sync?since=<epoch>` — pull message changes
- `POST /api/v1/sync` — push message changes
- `GET /api/v1/sync/contacts?since=<epoch>` — pull contact changes
- `POST /api/v1/sync/contacts` — push contact changes

### Auth API
- `POST /api/v1/auth/login` — authenticate, returns session token
- `POST /api/v1/auth/register` — create account
- `POST /api/v1/auth/logout` — invalidate session
- `GET /api/v1/auth/profile` — get current user profile (Bearer auth)
- `PUT /api/v1/auth/profile` — update profile
- `POST /api/v1/auth/change-password` — change password
- `POST /api/v1/auth/api-keys` — create API key
- `GET /api/v1/auth/api-keys` — list API keys
- `DELETE /api/v1/auth/api-keys/{id}` — delete API key

### User Admin API (admin only)
- `GET /api/v1/users` — list users
- `GET /api/v1/users/count` — count users
- `PUT /api/v1/users/{id}/enabled` — enable/disable user
- `PUT /api/v1/users/{id}/role` — change user role

### Web UI
- `GET /auth` — login page
- `GET /` — home dashboard
- `GET /contacts` — contacts list
- `GET /admin/users` — user management (admin)
- `GET /settings` — user settings
- `GET /health` — health check
- `GET /metrics` — Prometheus metrics

## Tech Stack

- **Go 1.24+** — single static binary, no runtime
- **chi v5** — HTTP router
- **pgx v5 + sqlc** — PostgreSQL with compile-time type-safe queries
- **viper** — YAML configuration with env overrides
- **golang-jwt** — JWT token support
- **bcrypt** — password hashing
- **prometheus** — metrics
- **slog** — structured logging

## License

[MIT](LICENSE)
