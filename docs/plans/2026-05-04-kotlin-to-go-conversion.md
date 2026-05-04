# Kotlin-to-Go Conversion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the outerstellar-platform web server and outerstellar-framework libraries from Kotlin/JVM to Go, producing a single static binary with ~10-15x lower RAM usage and ~50x faster cold start, while preserving the exact same HTTP API contract so the existing Java Swing desktop app continues to work unchanged.

**Architecture:** Go monorepo using chi router, pgx+sqlc for PostgreSQL, viper for YAML config, manual constructor DI, and `html/template` for server-side rendering. The Swing desktop app (Kotlin/JVM) is untouched -- it talks to the same sync/auth API endpoints over HTTP.

**Tech Stack:** Go 1.24+, chi v5, pgx v5, sqlc, golang-migrate, viper, golang-jwt, bcrypt, go-cache, prometheus client_golang, otel-go, gobreaker, slog, testify, testcontainers-go

---

## File Structure

```
gouterstellar-platform/
├── go.mod
├── go.sum
├── sqlc.yaml
├── Makefile
├── config/
│   ├── application.yaml
│   └── application-dev.yaml
├── migrations/
│   ├── V1__initial_schema.sql          (copied from Kotlin repo)
│   ├── V2__user_profile_enhancements.sql
│   ├── V3__sessions_table.sql
│   └── V4__user_preferences.sql
├── queries/
│   ├── messages.sql                    (sqlc query definitions)
│   ├── contacts.sql
│   ├── users.sql
│   ├── sessions.sql
│   ├── api_keys.sql
│   ├── outbox.sql
│   ├── audit.sql
│   ├── notifications.sql
│   ├── device_tokens.sql
│   ├── oauth_connections.sql
│   └── sync_state.sql
├── internal/
│   ├── model/
│   │   ├── message.go
│   │   ├── contact.go
│   │   ├── user.go
│   │   ├── auth.go
│   │   ├── pagination.go
│   │   ├── sync.go
│   │   ├── notification.go
│   │   ├── audit.go
│   │   ├── apikey.go
│   │   ├── errors.go
│   │   └── theme.go
│   ├── persistence/
│   │   ├── db/                         (sqlc generated — do not edit)
│   │   │   └── ...
│   │   ├── repository.go              (interfaces)
│   │   ├── message_repo.go
│   │   ├── contact_repo.go
│   │   ├── user_repo.go
│   │   ├── session_repo.go
│   │   ├── apikey_repo.go
│   │   ├── outbox_repo.go
│   │   ├── audit_repo.go
│   │   ├── notification_repo.go
│   │   ├── device_token_repo.go
│   │   ├── oauth_repo.go
│   │   ├── sync_state_repo.go
│   │   ├── tx.go                      (TransactionManager)
│   │   └── cache.go                   (go-cache wrapper)
│   ├── service/
│   │   ├── message_service.go
│   │   ├── contact_service.go
│   │   ├── security_service.go
│   │   ├── outbox_processor.go
│   │   ├── notification_service.go
│   │   ├── email_service.go
│   │   ├── smtp_email_service.go
│   │   ├── resilient_email_service.go
│   │   ├── push_notification_service.go
│   │   ├── analytics_service.go
│   │   ├── segment_analytics_service.go
│   │   ├── event_publisher.go
│   │   └── password_reset_service.go
│   ├── security/
│   │   ├── password_encoder.go
│   │   ├── jwt_service.go
│   │   ├── auth_realm.go
│   │   ├── permission.go
│   │   ├── apikey_service.go
│   │   ├── oauth_service.go
│   │   ├── apple_oauth_provider.go
│   │   └── async_activity_updater.go
│   ├── web/
│   │   ├── handler/
│   │   │   ├── home.go
│   │   │   ├── auth.go
│   │   │   ├── auth_api.go
│   │   │   ├── sync_api.go
│   │   │   ├── contacts.go
│   │   │   ├── user_admin.go
│   │   │   ├── user_admin_api.go
│   │   │   ├── notifications.go
│   │   │   ├── notification_api.go
│   │   │   ├── device_registration_api.go
│   │   │   ├── oauth.go
│   │   │   ├── search.go
│   │   │   ├── settings.go
│   │   │   ├── errors.go
│   │   │   ├── dev_dashboard.go
│   │   │   ├── components.go
│   │   │   └── sync_websocket.go
│   │   ├── filter/
│   │   │   ├── cors.go
│   │   │   ├── csrf.go
│   │   │   ├── security_headers.go
│   │   │   ├── session.go
│   │   │   ├── auth.go
│   │   │   ├── logging.go
│   │   │   ├── telemetry.go
│   │   │   ├── rate_limiter.go
│   │   │   ├── etag.go
│   │   │   ├── metrics.go
│   │   │   └── error_handler.go
│   │   ├── template/
│   │   │   ├── layouts/
│   │   │   │   ├── layout_head.html
│   │   │   │   ├── sidebar.html
│   │   │   │   ├── topbar.html
│   │   │   │   └── layout_router.html
│   │   │   ├── pages/
│   │   │   │   ├── home.html
│   │   │   │   ├── auth.html
│   │   │   │   ├── contacts.html
│   │   │   │   ├── user_admin.html
│   │   │   │   ├── notifications.html
│   │   │   │   ├── search.html
│   │   │   │   ├── settings.html
│   │   │   │   ├── error.html
│   │   │   │   ├── dev_dashboard.html
│   │   │   │   ├── reset_password.html
│   │   │   │   ├── change_password.html
│   │   │   │   ├── trash.html
│   │   │   │   ├── api_keys.html
│   │   │   │   ├── profile.html
│   │   │   │   ├── audit_log.html
│   │   │   │   └── auth_result.html
│   │   │   └── components/
│   │   │       ├── message_list.html
│   │   │       ├── pagination.html
│   │   │       ├── page_header.html
│   │   │       ├── notification_bell.html
│   │   │       ├── modal.html
│   │   │       ├── contact_form.html
│   │   │       ├── conflict_resolve_modal.html
│   │   │       ├── sidebar_selector.html
│   │   │       └── footer_status.html
│   │   ├── viewmodel/
│   │   │   ├── shell.go
│   │   │   ├── page.go
│   │   │   └── viewmodels.go
│   │   ├── context.go                (WebContext per-request state)
│   │   ├── session_cookie.go
│   │   ├── theme_catalog.go
│   │   └── renderer.go               (template loading + rendering)
│   ├── config/
│   │   └── config.go
│   └── wire/
│       └── wire.go                    (manual constructor DI — all wiring in one place)
├── pkg/
│   ├── i18n/
│   │   ├── i18n.go
│   │   ├── language.go
│   │   └── i18n_test.go
│   ├── theme/
│   │   ├── theme.go
│   │   ├── shader.go
│   │   └── theme_test.go
│   └── plugin/
│       ├── plugin.go
│       ├── manager.go
│       └── manager_test.go
├── cmd/
│   ├── server/
│   │   └── main.go
│   ├── seed/
│   │   └── main.go
│   └── i18n-validate/
│       └── main.go
└── static/
    ├── site.css
    ├── platform.js
    └── vendor/
        ├── htmx.min.js
        ├── htmx-ext-ws.js
        └── remixicon/
```

---

## Task 1: Project Scaffold + go.mod

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `sqlc.yaml`
- Create: `config/application.yaml`
- Create: `config/application-dev.yaml`

- [ ] **Step 1: Initialize Go module and create go.mod**

```go
// go.mod
module github.com/rygel/gouterstellar-platform

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/go-playground/validator/v10 v10.26.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/golang-migrate/migrate/v4 v4.18.3
	github.com/jackc/pgx/v5 v5.7.4
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/prometheus/client_golang v1.22.0
	github.com/sony/gobreaker v1.0.0
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.21.0
	github.com/testcontainers/testcontainers-go v0.37.0
	github.com/wneessen/go-mail v0.6.2
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0
	go.opentelemetry.io/otel/sdk v1.35.0
	golang.org/x/crypto v0.38.0
)
```

Run: `go mod tidy`

- [ ] **Step 2: Create Makefile**

```makefile
.PHONY: build test lint generate clean migrate-up migrate-down

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -timeout 120s -count=1

lint:
	golangci-lint run ./...

generate:
	sqlc generate

clean:
	rm -rf bin/

migrate-up:
	migrate -path migrations -database "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable" down 1

dev:
	go run ./cmd/server

seed:
	go run ./cmd/seed
```

- [ ] **Step 3: Create sqlc.yaml**

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "migrations/"
    gen:
      go:
        package: "db"
        out: "internal/persistence/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        emit_interface: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "timestamp"
            go_type: "time.Time"
```

- [ ] **Step 4: Create config files**

```yaml
# config/application.yaml
version: "dev"
port: 8080
database_url: "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable"
dev_dashboard_enabled: false
dev_mode: false
session_cookie_secure: false
session_timeout_minutes: 30
cors_origins: "*"
csrf_enabled: true
app_base_url: "http://localhost:8080"
csp_policy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:;"

jwt:
  enabled: false
  secret: ""
  issuer: "outerstellar"
  expiry_seconds: 86400

email:
  enabled: false
  host: "localhost"
  port: 587
  username: ""
  password: ""
  from: "noreply@example.com"
  starttls: true

segment:
  write_key: ""
  enabled: false
```

```yaml
# config/application-dev.yaml
dev_dashboard_enabled: true
dev_mode: true
```

- [ ] **Step 5: Create directory structure and placeholder files**

Create all directories from the file structure above. Each `.go` file gets a `package` declaration only.

Run:
```bash
mkdir -p internal/model internal/persistence/db internal/service internal/security
mkdir -p internal/web/handler internal/web/filter internal/web/template/layouts
mkdir -p internal/web/template/pages internal/web/template/components
mkdir -p internal/web/viewmodel internal/config internal/wire
mkdir -p pkg/i18n pkg/theme pkg/plugin
mkdir -p cmd/server cmd/seed cmd/i18n-validate
mkdir -p queries migrations static/vendor
```

- [ ] **Step 6: Copy migration files**

Copy V1-V4 SQL files verbatim from `platform-persistence-jooq/src/main/resources/db/migration/` to `migrations/`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: scaffold Go monorepo with go.mod, Makefile, sqlc config, and migration files"
```

---

## Task 2: Domain Models

**Files:**
- Create: `internal/model/message.go`
- Create: `internal/model/contact.go`
- Create: `internal/model/user.go`
- Create: `internal/model/auth.go`
- Create: `internal/model/pagination.go`
- Create: `internal/model/sync.go`
- Create: `internal/model/notification.go`
- Create: `internal/model/audit.go`
- Create: `internal/model/apikey.go`
- Create: `internal/model/errors.go`
- Create: `internal/model/theme.go`
- Create: `internal/model/errors_test.go`
- Test: `internal/model/errors_test.go`

- [ ] **Step 1: Write tests for error types**

```go
// internal/model/errors_test.go
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConflictStrategyFromString(t *testing.T) {
	assert.Equal(t, ConflictMine, ConflictStrategyFromString("mine"))
	assert.Equal(t, ConflictServer, ConflictStrategyFromString("server"))
	assert.Equal(t, ConflictServer, ConflictStrategyFromString("unknown"))
}

func TestPermissionParse(t *testing.T) {
	p := Permission{Domain: "admin"}
	assert.Equal(t, p, ParsePermission("admin"))

	p2 := Permission{Domain: "message", Action: "read"}
	assert.Equal(t, p2, ParsePermission("message:read"))

	p3 := Permission{Domain: "message", Action: "read", Instance: "123"}
	assert.Equal(t, p3, ParsePermission("message:read:123"))
}

func TestPermissionImplies(t *testing.T) {
	super := Permission{Domain: "*", Action: "*"}
	assert.True(t, super.Implies(Permission{Domain: "message", Action: "read", Instance: "123"}))

	domain := Permission{Domain: "message", Action: "*"}
	assert.True(t, domain.Implies(Permission{Domain: "message", Action: "read"}))
	assert.False(t, domain.Implies(Permission{Domain: "contact", Action: "read"}))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestConflict -v`
Expected: FAIL — types not defined yet

- [ ] **Step 3: Create all model files**

```go
// internal/model/errors.go
package model

import "fmt"

type ConflictStrategy int

const (
	ConflictMine ConflictStrategy = iota
	ConflictServer
)

func ConflictStrategyFromString(value string) ConflictStrategy {
	switch value {
	case "mine":
		return ConflictMine
	default:
		return ConflictServer
	}
}

type OuterstellarError struct {
	Message string
	Cause   error
}

func (e *OuterstellarError) Error() string { return e.Message }

type MessageNotFoundError struct {
	SyncID string
}

func (e *MessageNotFoundError) Error() string {
	return fmt.Sprintf("Message with sync ID %s was not found.", e.SyncID)
}

type ContactNotFoundError struct {
	SyncID string
}

func (e *ContactNotFoundError) Error() string {
	return fmt.Sprintf("Contact with sync ID %s was not found.", e.SyncID)
}

type DuplicateMessageError struct {
	SyncID string
}

func (e *DuplicateMessageError) Error() string {
	return fmt.Sprintf("A message with sync ID %s already exists.", e.SyncID)
}

type SyncConflictError struct {
	SyncID string
	Reason string
}

func (e *SyncConflictError) Error() string {
	return fmt.Sprintf("Sync conflict for message %s: %s", e.SyncID, e.Reason)
}

type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Validation failed: %v", e.Errors)
}

type OptimisticLockError struct {
	EntityType string
	SyncID     string
}

func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("%s with sync ID %s was modified by another process.", e.EntityType, e.SyncID)
}

type SyncError struct {
	Message string
	Cause   error
}

func (e *SyncError) Error() string { return e.Message }

type UsernameAlreadyExistsError struct {
	Username string
}

func (e *UsernameAlreadyExistsError) Error() string {
	return fmt.Sprintf("Username '%s' is already taken.", e.Username)
}

type WeakPasswordError struct {
	Message string
}

func (e *WeakPasswordError) Error() string { return e.Message }

type UserNotFoundError struct {
	UserID string
}

func (e *UserNotFoundError) Error() string {
	return fmt.Sprintf("User with ID %s was not found.", e.UserID)
}

type InsufficientPermissionError struct {
	Message string
}

func (e *InsufficientPermissionError) Error() string { return e.Message }

type SessionExpiredError struct{}

func (e *SessionExpiredError) Error() string { return "Session has expired" }
```

```go
// internal/model/message.go
package model

import "time"

type StoredMessage struct {
	SyncID           string
	Author           string
	Content          string
	UpdatedAtEpochMs int64
	Dirty            bool
	Deleted          bool
	Version          int64
	SyncConflict     *string
}

type MessageSummary struct {
	SyncID           string
	Author           string
	Content          string
	UpdatedAtEpochMs int64
	Dirty            bool
	Version          int64
	HasConflict      bool
}

func (m *MessageSummary) UpdatedAtLabel() string {
	return time.UnixMilli(m.UpdatedAtEpochMs).Format("2006-01-02 15:04")
}
```

```go
// internal/model/contact.go
package model

type StoredContact struct {
	SyncID           string
	Name             string
	Emails           []string
	Phones           []string
	SocialMedia      []string
	Company          string
	CompanyAddress   string
	Department       string
	UpdatedAtEpochMs int64
	Dirty            bool
	Deleted          bool
	Version          int64
	SyncConflict     *string
}

type ContactSummary struct {
	SyncID           string
	Name             string
	Emails           []string
	Phones           []string
	SocialMedia      []string
	Company          string
	CompanyAddress   string
	Department       string
	UpdatedAtEpochMs int64
	Dirty            bool
	Version          int64
	HasConflict      bool
}
```

```go
// internal/model/user.go
package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

type User struct {
	ID                        uuid.UUID
	Username                  string
	Email                     string
	PasswordHash              string
	Role                      UserRole
	Enabled                   bool
	LastActivityAt            *time.Time
	AvatarURL                 *string
	EmailNotificationsEnabled bool
	PushNotificationsEnabled  bool
	Language                  *string
	Theme                     *string
	Layout                    *string
}

type UserSummary struct {
	ID       string
	Username string
	Email    string
	Role     string
	Enabled  bool
}

func (u *User) ToSummary() UserSummary {
	return UserSummary{
		ID: u.ID.String(), Username: u.Username,
		Email: u.Email, Role: string(u.Role), Enabled: u.Enabled,
	}
}
```

```go
// internal/model/auth.go
package model

import "time"

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type AuthTokenResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required"`
}

type UpdateProfileRequest struct {
	Email     string  `json:"email" validate:"required,email"`
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
}

type UpdateNotificationPrefsRequest struct {
	EmailEnabled bool `json:"emailEnabled"`
	PushEnabled  bool `json:"pushEnabled"`
}

type UserProfileResponse struct {
	Username                  string  `json:"username"`
	Email                     string  `json:"email"`
	AvatarURL                 *string `json:"avatarUrl"`
	EmailNotificationsEnabled bool    `json:"emailNotificationsEnabled"`
	PushNotificationsEnabled  bool    `json:"pushNotificationsEnabled"`
}

type SetUserEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type SetUserRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

type CreateApiKeyRequest struct {
	Name string `json:"name" validate:"required"`
}

type CreateApiKeyResponse struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	KeyPrefix string `json:"keyPrefix"`
}

type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type PasswordResetConfirm struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required"`
}
```

```go
// internal/model/pagination.go
package model

import "math"

type PaginationMetadata struct {
	CurrentPage  int
	PageSize     int
	TotalItems   int64
	TotalPages   int
	HasPrevious  bool
	HasNext      bool
	PreviousPage *int
	NextPage     *int
}

func NewPaginationMetadata(currentPage, pageSize int, totalItems int64) PaginationMetadata {
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	md := PaginationMetadata{
		CurrentPage: currentPage, PageSize: pageSize,
		TotalItems: totalItems, TotalPages: totalPages,
		HasPrevious: currentPage > 1, HasNext: currentPage < totalPages,
	}
	if md.HasPrevious {
		p := currentPage - 1
		md.PreviousPage = &p
	}
	if md.HasNext {
		n := currentPage + 1
		md.NextPage = &n
	}
	return md
}

type PagedResult[T any] struct {
	Items    []T
	Metadata PaginationMetadata
}
```

```go
// internal/model/sync.go
package model

import "encoding/json"

type SyncMessage struct {
	SyncID           string `json:"syncId"`
	Author           string `json:"author"`
	Content          string `json:"content"`
	UpdatedAtEpochMs int64  `json:"updatedAtEpochMs"`
	Deleted          bool   `json:"deleted"`
}

type SyncPushRequest struct {
	Messages []SyncMessage `json:"messages"`
}

type SyncConflict struct {
	SyncID        string       `json:"syncId"`
	Reason        string       `json:"reason"`
	ServerMessage *SyncMessage `json:"serverMessage"`
}

type SyncPushResponse struct {
	AppliedCount int            `json:"appliedCount"`
	Conflicts    []SyncConflict `json:"conflicts"`
}

type SyncPullResponse struct {
	Messages       []SyncMessage `json:"messages"`
	ServerTimestamp int64         `json:"serverTimestamp"`
}

type SyncStats struct {
	PushedCount  int `json:"pushedCount"`
	PulledCount  int `json:"pulledCount"`
	ConflictCount int `json:"conflictCount"`
}

type SyncContact struct {
	SyncID           string   `json:"syncId"`
	Name             string   `json:"name"`
	Emails           []string `json:"emails"`
	Phones           []string `json:"phones"`
	SocialMedia      []string `json:"socialMedia"`
	Company          string   `json:"company"`
	CompanyAddress   string   `json:"companyAddress"`
	Department       string   `json:"department"`
	UpdatedAtEpochMs int64    `json:"updatedAtEpochMs"`
	Deleted          bool     `json:"deleted"`
}

type SyncPushContactRequest struct {
	Contacts []SyncContact `json:"contacts"`
}

type SyncContactConflict struct {
	SyncID        string       `json:"syncId"`
	Reason        string       `json:"reason"`
	ServerContact *SyncContact `json:"serverContact"`
}

type SyncPushContactResponse struct {
	AppliedCount int                   `json:"appliedCount"`
	Conflicts    []SyncContactConflict `json:"conflicts"`
}

type SyncPullContactResponse struct {
	Contacts        []SyncContact `json:"contacts"`
	ServerTimestamp int64         `json:"serverTimestamp"`
}

func SyncMessageToJSON(msg SyncMessage) (string, error) {
	b, err := json.Marshal(msg)
	return string(b), err
}

func SyncMessageFromJSON(data string) (SyncMessage, error) {
	var msg SyncMessage
	err := json.Unmarshal([]byte(data), &msg)
	return msg, err
}

func SyncContactToJSON(c SyncContact) (string, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func SyncContactFromJSON(data string) (SyncContact, error) {
	var c SyncContact
	err := json.Unmarshal([]byte(data), &c)
	return c, err
}
```

```go
// internal/model/notification.go
package model

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"userId"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Type      string     `json:"type"`
	ReadAt    *time.Time `json:"readAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

func (n *Notification) IsRead() bool { return n.ReadAt != nil }

type NotificationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Type      string `json:"type"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}
```

```go
// internal/model/audit.go
package model

import "time"

type AuditEntry struct {
	ID             int64      `json:"id"`
	ActorID        *string    `json:"actorId"`
	ActorUsername  *string    `json:"actorUsername"`
	TargetID       *string    `json:"targetId"`
	TargetUsername *string    `json:"targetUsername"`
	Action         string     `json:"action"`
	Detail         *string    `json:"detail"`
	CreatedAt      time.Time  `json:"createdAt"`
}
```

```go
// internal/model/apikey.go
package model

import "time"

type ApiKey struct {
	ID         int64      `json:"id"`
	UserID     string     `json:"userId"`
	KeyHash    string     `json:"keyHash"`
	KeyPrefix  string     `json:"keyPrefix"`
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

type ApiKeySummary struct {
	ID         int64   `json:"id"`
	KeyPrefix  string  `json:"keyPrefix"`
	Name       string  `json:"name"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
}
```

```go
// internal/model/theme.go
package model

type ThemeDefinition struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Colors map[string]string `json:"colors"`
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/model/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/model/
git commit -m "feat: add domain models — messages, contacts, users, auth, sync, pagination, errors"
```

---

## Task 3: Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write test for config loading**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	cfg := Load()
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "postgres://test:test@localhost:5432/testdb", cfg.DatabaseURL)
	assert.False(t, cfg.SessionCookieSecure)
	assert.Equal(t, 30, cfg.SessionTimeoutMinutes)
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("SESSION_TIMEOUT_MINUTES", "60")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("SESSION_TIMEOUT_MINUTES")

	cfg := Load()
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, 60, cfg.SessionTimeoutMinutes)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL

- [ ] **Step 3: Implement config**

```go
// internal/config/config.go
package config

import (
	"log/slog"

	"github.com/spf13/viper"
)

type JwtConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Secret       string `mapstructure:"secret"`
	Issuer       string `mapstructure:"issuer"`
	ExpirySeconds int64 `mapstructure:"expiry_seconds"`
}

type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	StartTLS bool   `mapstructure:"starttls"`
}

type SegmentConfig struct {
	WriteKey string `mapstructure:"write_key"`
	Enabled  bool   `mapstructure:"enabled"`
}

type Config struct {
	Version              string       `mapstructure:"version"`
	Port                 int          `mapstructure:"port"`
	DatabaseURL          string       `mapstructure:"database_url"`
	DevDashboardEnabled  bool         `mapstructure:"dev_dashboard_enabled"`
	DevMode              bool         `mapstructure:"dev_mode"`
	SessionCookieSecure  bool         `mapstructure:"session_cookie_secure"`
	SessionTimeoutMinutes int         `mapstructure:"session_timeout_minutes"`
	CORSOrigins          string       `mapstructure:"cors_origins"`
	CSRFEnabled          bool         `mapstructure:"csrf_enabled"`
	AppBaseURL           string       `mapstructure:"app_base_url"`
	CSPPolicy            string       `mapstructure:"csp_policy"`
	JWT                  JwtConfig    `mapstructure:"jwt"`
	Email                EmailConfig  `mapstructure:"email"`
	Segment              SegmentConfig `mapstructure:"segment"`
}

func Load() *Config {
	v := viper.New()

	v.SetConfigName("application")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")
	v.AddConfigPath(".")

	v.SetEnvPrefix("")
	v.AutomaticEnv()

	v.SetDefault("version", "dev")
	v.SetDefault("port", 8080)
	v.SetDefault("database_url", "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable")
	v.SetDefault("dev_dashboard_enabled", false)
	v.SetDefault("dev_mode", false)
	v.SetDefault("session_cookie_secure", false)
	v.SetDefault("session_timeout_minutes", 30)
	v.SetDefault("cors_origins", "*")
	v.SetDefault("csrf_enabled", true)
	v.SetDefault("app_base_url", "http://localhost:8080")

	if err := v.ReadInConfig(); err != nil {
		slog.Warn("No config file found, using defaults and env vars", "error", err)
	}

	profile := v.GetString("APP_PROFILE")
	if profile != "" && profile != "default" {
		v.SetConfigName("application-" + profile)
		_ = v.MergeInConfig()
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		slog.Error("Failed to unmarshal config", "error", err)
		panic(err)
	}
	return &cfg
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add viper-based YAML config with env overrides"
```

---

## Task 4: Persistence Layer — sqlc Queries + Repository Interfaces

**Files:**
- Create: `queries/messages.sql`
- Create: `queries/contacts.sql`
- Create: `queries/users.sql`
- Create: `queries/sessions.sql`
- Create: `queries/api_keys.sql`
- Create: `queries/outbox.sql`
- Create: `queries/audit.sql`
- Create: `queries/notifications.sql`
- Create: `queries/device_tokens.sql`
- Create: `queries/oauth_connections.sql`
- Create: `queries/sync_state.sql`
- Create: `internal/persistence/repository.go`
- Create: `internal/persistence/tx.go`
- Create: `internal/persistence/cache.go`

- [ ] **Step 1: Write sqlc query files**

Each file contains annotated SQL matching the `plt_` prefixed tables from V1-V4 migrations. Example for messages:

```sql
-- queries/messages.sql
-- name: ListMessages :many
SELECT sync_id, author, content, updated_at_epoch_ms, dirty, version, sync_conflict
FROM plt_messages
WHERE deleted = false
  AND ($1::text IS NULL OR author ILIKE '%' || $1 || '%' OR content ILIKE '%' || $1 || '%')
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMessages :one
SELECT COUNT(*) FROM plt_messages
WHERE deleted = false
  AND ($1::text IS NULL OR author ILIKE '%' || $1 || '%' OR content ILIKE '%' || $1 || '%');

-- name: FindBySyncID :one
SELECT sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict
FROM plt_messages WHERE sync_id = $1;

-- name: CreateServerMessage :one
INSERT INTO plt_messages (sync_id, author, content, dirty)
VALUES (gen_random_uuid()::text, $1, $2, false)
RETURNING sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict;

-- name: CreateLocalMessage :one
INSERT INTO plt_messages (sync_id, author, content, dirty)
VALUES (gen_random_uuid()::text, $1, $2, true)
RETURNING sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict;

-- name: UpsertSyncedMessage :one
INSERT INTO plt_messages (sync_id, author, content, updated_at_epoch_ms, deleted, dirty, version)
VALUES ($1, $2, $3, $4, $5, $6, 1)
ON CONFLICT (sync_id) DO UPDATE SET
    author = EXCLUDED.author,
    content = EXCLUDED.content,
    updated_at_epoch_ms = EXCLUDED.updated_at_epoch_ms,
    deleted = EXCLUDED.deleted,
    dirty = EXCLUDED.dirty,
    version = plt_messages.version + 1
RETURNING sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict;

-- name: FindChangesSince :many
SELECT sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict
FROM plt_messages
WHERE updated_at_epoch_ms > $1
ORDER BY updated_at_epoch_ms ASC;

-- name: ListDirtyMessages :many
SELECT sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict
FROM plt_messages WHERE dirty = true AND deleted = false;

-- name: CountDirtyMessages :one
SELECT COUNT(*) FROM plt_messages WHERE dirty = true AND deleted = false;

-- name: SoftDeleteMessage :exec
UPDATE plt_messages SET deleted = true, deleted_at = NOW() WHERE sync_id = $1;

-- name: RestoreMessage :exec
UPDATE plt_messages SET deleted = false, deleted_at = NULL WHERE sync_id = $1;

-- name: UpdateMessage :one
UPDATE plt_messages SET author = $2, content = $3, updated_at_epoch_ms = $4,
    dirty = $5, sync_conflict = $6
WHERE sync_id = $1
RETURNING sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version, sync_conflict;

-- name: MarkConflictMessage :exec
UPDATE plt_messages SET sync_conflict = $2 WHERE sync_id = $1;

-- name: ResolveConflictMessage :exec
UPDATE plt_messages SET author = $2, content = $3, updated_at_epoch_ms = $4,
    dirty = $5, sync_conflict = NULL, version = version + 1
WHERE sync_id = $1;

-- name: MarkCleanMessages :exec
UPDATE plt_messages SET dirty = false WHERE sync_id = ANY($1::text[]);
```

(Write equivalent files for contacts, users, sessions, api_keys, outbox, audit, notifications, device_tokens, oauth_connections, sync_state — each matching the table schema from V1-V4.)

- [ ] **Step 2: Run sqlc generate**

Run: `sqlc generate`
Expected: Generated Go code in `internal/persistence/db/`

- [ ] **Step 3: Create repository interfaces**

```go
// internal/persistence/repository.go
package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rygel/gouterstellar-platform/internal/model"
)

type MessageRepository interface {
	ListMessages(ctx context.Context, query *string, year *int, limit, offset int, includeDeleted bool) ([]model.MessageSummary, error)
	CountMessages(ctx context.Context, query *string, year *int, includeDeleted bool) (int64, error)
	FindBySyncID(ctx context.Context, syncID string) (*model.StoredMessage, error)
	CreateServerMessage(ctx context.Context, author, content string) (*model.StoredMessage, error)
	CreateLocalMessage(ctx context.Context, author, content string) (*model.StoredMessage, error)
	UpsertSyncedMessage(ctx context.Context, msg model.SyncMessage, dirty bool) (*model.StoredMessage, error)
	FindChangesSince(ctx context.Context, since int64) ([]model.StoredMessage, error)
	ListDirtyMessages(ctx context.Context) ([]model.StoredMessage, error)
	CountDirtyMessages(ctx context.Context) (int64, error)
	SoftDelete(ctx context.Context, syncID string) error
	Restore(ctx context.Context, syncID string) error
	UpdateMessage(ctx context.Context, msg model.StoredMessage) (*model.StoredMessage, error)
	MarkConflict(ctx context.Context, syncID string, serverVersion model.SyncMessage) error
	ResolveConflict(ctx context.Context, syncID string, resolved model.StoredMessage) error
	MarkClean(ctx context.Context, syncIDs []string) error
	GetLastSyncEpochMs(ctx context.Context) (int64, error)
	SetLastSyncEpochMs(ctx context.Context, value int64) error
}

type ContactRepository interface {
	ListContacts(ctx context.Context, query *string, limit, offset int, includeDeleted bool) ([]model.ContactSummary, error)
	CountContacts(ctx context.Context, query *string, includeDeleted bool) (int64, error)
	ListDirtyContacts(ctx context.Context) ([]model.StoredContact, error)
	FindBySyncID(ctx context.Context, syncID string) (*model.StoredContact, error)
	FindChangesSince(ctx context.Context, since int64) ([]model.StoredContact, error)
	CreateServerContact(ctx context.Context, name string, emails, phones, socialMedia []string, company, companyAddress, department string) (*model.StoredContact, error)
	CreateLocalContact(ctx context.Context, name string, emails, phones, socialMedia []string, company, companyAddress, department string) (*model.StoredContact, error)
	UpsertSyncedContact(ctx context.Context, contact model.SyncContact, dirty bool) (*model.StoredContact, error)
	MarkClean(ctx context.Context, syncIDs []string) error
	GetLastSyncEpochMs(ctx context.Context) (int64, error)
	SetLastSyncEpochMs(ctx context.Context, value int64) error
	SoftDelete(ctx context.Context, syncID string) error
	Restore(ctx context.Context, syncID string) error
	UpdateContact(ctx context.Context, contact model.StoredContact) (*model.StoredContact, error)
	MarkConflict(ctx context.Context, syncID string, serverVersion model.SyncContact) error
	ResolveConflict(ctx context.Context, syncID string, resolved model.StoredContact) error
}

type UserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	Save(ctx context.Context, user model.User) error
	FindAll(ctx context.Context) ([]model.User, error)
	FindPage(ctx context.Context, limit, offset int) ([]model.User, error)
	CountAll(ctx context.Context) (int64, error)
	CountByRole(ctx context.Context, role model.UserRole) (int64, error)
	UpdateRole(ctx context.Context, userID uuid.UUID, role model.UserRole) error
	UpdateEnabled(ctx context.Context, userID uuid.UUID, enabled bool) error
	UpdateLastActivity(ctx context.Context, userID uuid.UUID) error
	DeleteByID(ctx context.Context, userID uuid.UUID) error
	UpdateUsername(ctx context.Context, userID uuid.UUID, newUsername string) error
	UpdateAvatarURL(ctx context.Context, userID uuid.UUID, avatarURL *string) error
	UpdateNotificationPreferences(ctx context.Context, userID uuid.UUID, emailEnabled, pushEnabled bool) error
	UpdatePreferences(ctx context.Context, userID uuid.UUID, language, theme, layout *string) error
	SeedAdminUser(ctx context.Context, passwordHash string) error
}

type SessionRepository interface {
	Save(ctx context.Context, session model.Session) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	FindByTokenHashIncludingExpired(ctx context.Context, tokenHash string) (*model.Session, error)
	UpdateExpiresAt(ctx context.Context, tokenHash string, expiresAt time.Time) error
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type ApiKeyRepository interface {
	Save(ctx context.Context, apiKey model.ApiKey) error
	FindByKeyHash(ctx context.Context, keyHash string) (*model.ApiKey, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.ApiKey, error)
	Delete(ctx context.Context, id int64, userID uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id int64) error
}

type OutboxRepository interface {
	Save(ctx context.Context, entry model.OutboxEntry) error
	ListPending(ctx context.Context, limit int) ([]model.OutboxEntry, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
	GetStats(ctx context.Context) (map[string]int, error)
	ListFailed(ctx context.Context) ([]model.OutboxEntry, error)
}

type AuditRepository interface {
	Log(ctx context.Context, entry model.AuditEntry) error
	FindRecent(ctx context.Context, limit int) ([]model.AuditEntry, error)
	FindPage(ctx context.Context, limit, offset int) ([]model.AuditEntry, error)
	CountAll(ctx context.Context) (int64, error)
}

type NotificationRepository interface {
	Save(ctx context.Context, notification model.Notification) error
	FindByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.Notification, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type DeviceTokenRepository interface {
	Upsert(ctx context.Context, token model.DeviceToken) error
	Delete(ctx context.Context, token string) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error)
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

type OAuthRepository interface {
	FindByProviderSubject(ctx context.Context, provider, subject string) (*model.OAuthConnection, error)
	Save(ctx context.Context, connection model.OAuthConnection) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.OAuthConnection, error)
	Delete(ctx context.Context, id int64, userID uuid.UUID) error
}

type PasswordResetRepository interface {
	Save(ctx context.Context, token model.PasswordResetToken) error
	FindByToken(ctx context.Context, token string) (*model.PasswordResetToken, error)
	MarkUsed(ctx context.Context, token string) error
}
```

- [ ] **Step 4: Create TransactionManager and Cache**

```go
// internal/persistence/tx.go
package persistence

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionManager struct {
	pool *pgxpool.Pool
}

func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{pool: pool}
}

func (tm *TransactionManager) InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (tm *TransactionManager) Pool() *pgxpool.Pool {
	return tm.pool
}
```

```go
// internal/persistence/cache.go
package persistence

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type MessageCache struct {
	inner *cache.Cache
}

func NewMessageCache(defaultTTL time.Duration) *MessageCache {
	return &MessageCache{inner: cache.New(defaultTTL, defaultTTL*2)}
}

func (c *MessageCache) Get(key string) (interface{}, bool) {
	return c.inner.Get(key)
}

func (c *MessageCache) Set(key string, value interface{}) {
	c.inner.Set(key, value, cache.DefaultExpiration)
}

func (c *MessageCache) GetOrSet(key string, fn func() interface{}) interface{} {
	if val, found := c.inner.Get(key); found {
		return val
	}
	val := fn()
	c.inner.Set(key, val, cache.DefaultExpiration)
	return val
}

func (c *MessageCache) Invalidate(key string) {
	c.inner.Delete(key)
}

func (c *MessageCache) InvalidateAll() {
	c.inner.Flush()
}

func (c *MessageCache) InvalidateByPrefix(prefix string) {
	for key := range c.inner.Items() {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.inner.Delete(key)
		}
	}
}
```

- [ ] **Step 5: Commit**

```bash
git add queries/ internal/persistence/
git commit -m "feat: add sqlc queries, repository interfaces, transaction manager, and cache"
```

---

## Task 5: Repository Implementations

**Files:**
- Create: `internal/persistence/message_repo.go`
- Create: `internal/persistence/contact_repo.go`
- Create: `internal/persistence/user_repo.go`
- Create: `internal/persistence/session_repo.go`
- Create: `internal/persistence/apikey_repo.go`
- Create: `internal/persistence/outbox_repo.go`
- Create: `internal/persistence/audit_repo.go`
- Create: `internal/persistence/notification_repo.go`
- Create: `internal/persistence/device_token_repo.go`
- Create: `internal/persistence/oauth_repo.go`
- Create: `internal/persistence/sync_state_repo.go`
- Create: `internal/persistence/password_reset_repo.go`

- [ ] **Step 1: Implement MessageRepository**

```go
// internal/persistence/message_repo.go
package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type messageRepo struct {
	q *db.Queries
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) MessageRepository {
	return &messageRepo{q: db.New(pool), pool: pool}
}

// Implement all MessageRepository methods, delegating to db.Queries.
// Map db generated structs to model.StoredMessage / model.MessageSummary.
// (Full implementation ~150 lines — straightforward field-by-field mapping.)
```

(Repeat for each repository. Each is a thin adapter between the sqlc-generated `db.Queries` types and the domain model types. ~100-200 lines each.)

- [ ] **Step 2: Write integration test with testcontainers**

```go
// internal/persistence/message_repo_test.go
package persistence

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()
	c, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { c.Terminate(ctx) })

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run migrations
	// ...

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	return pool
}

func TestMessageRepo_CreateAndFind(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewMessageRepository(pool)
	ctx := context.Background()

	msg, err := repo.CreateServerMessage(ctx, "alice", "Hello world")
	require.NoError(t, err)
	assert.Equal(t, "alice", msg.Author)
	assert.Equal(t, "Hello world", msg.Content)
	assert.False(t, msg.Dirty)

	found, err := repo.FindBySyncID(ctx, msg.SyncID)
	require.NoError(t, err)
	assert.Equal(t, msg.SyncID, found.SyncID)
}
```

- [ ] **Step 3: Run integration tests**

Run: `go test ./internal/persistence/ -v -tags integration`
Expected: PASS (requires Docker)

- [ ] **Step 4: Commit**

```bash
git add internal/persistence/
git commit -m "feat: implement all repository adapters over sqlc-generated queries"
```

---

## Task 6: Security Package

**Files:**
- Create: `internal/security/password_encoder.go`
- Create: `internal/security/jwt_service.go`
- Create: `internal/security/auth_realm.go`
- Create: `internal/security/permission.go`
- Create: `internal/security/apikey_service.go`
- Create: `internal/security/oauth_service.go`
- Create: `internal/security/apple_oauth_provider.go`
- Create: `internal/security/async_activity_updater.go`
- Create: `internal/security/password_encoder_test.go`
- Create: `internal/security/permission_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/security/password_encoder_test.go
package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBCryptPasswordEncoder(t *testing.T) {
	enc := NewBCryptPasswordEncoder(10)
	hash, err := enc.Encode("password123")
	assert.NoError(t, err)
	assert.True(t, enc.Matches("password123", hash))
	assert.False(t, enc.Matches("wrong", hash))
}

// internal/security/permission_test.go
// (already covered in model/errors_test.go but test the resolver too)

func TestRoleBasedPermissionResolver(t *testing.T) {
	resolver := NewRoleBasedPermissionResolver()
	admin := model.User{Role: model.RoleAdmin}
	user := model.User{Role: model.RoleUser}

	adminPerms := resolver.PermissionsFor(&admin)
	assert.Contains(t, adminPerms, model.Permission{Domain: "*", Action: "*"})

	userPerms := resolver.PermissionsFor(&user)
	assert.Contains(t, userPerms, model.Permission{Domain: "message", Action: "*"})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/security/ -v`

- [ ] **Step 3: Implement all security files**

Each file is a direct Go port of its Kotlin counterpart. Key examples:

```go
// internal/security/password_encoder.go
package security

import "golang.org/x/crypto/bcrypt"

type PasswordEncoder interface {
	Encode(password string) (string, error)
	Matches(password, hash string) bool
}

type bcryptEncoder struct{ cost int }

func NewBCryptPasswordEncoder(cost int) PasswordEncoder {
	return &bcryptEncoder{cost: cost}
}

func (e *bcryptEncoder) Encode(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), e.cost)
	return string(bytes), err
}

func (e *bcryptEncoder) Matches(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

```go
// internal/security/auth_realm.go
package security

import "github.com/rygel/gouterstellar-platform/internal/model"

type AuthResult interface{ authResult() }

type AuthenticatedResult struct{ User *model.User }
func (AuthenticatedResult) authResult() {}

type ExpiredResult struct{}
func (ExpiredResult) authResult() {}

type SkippedResult struct{}
func (SkippedResult) authResult() {}

type AuthRealm interface {
	Name() string
	Authenticate(token string) AuthResult
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/security/
git commit -m "feat: add security package — bcrypt, JWT, auth realms, permissions, API keys"
```

---

## Task 7: Service Layer

**Files:**
- Create: `internal/service/message_service.go`
- Create: `internal/service/contact_service.go`
- Create: `internal/service/security_service.go`
- Create: `internal/service/password_reset_service.go`
- Create: `internal/service/outbox_processor.go`
- Create: `internal/service/notification_service.go`
- Create: `internal/service/email_service.go`
- Create: `internal/service/smtp_email_service.go`
- Create: `internal/service/resilient_email_service.go`
- Create: `internal/service/push_notification_service.go`
- Create: `internal/service/analytics_service.go`
- Create: `internal/service/segment_analytics_service.go`
- Create: `internal/service/event_publisher.go`
- Create: `internal/service/message_service_test.go`
- Create: `internal/service/contact_service_test.go`
- Create: `internal/service/security_service_test.go`

- [ ] **Step 1: Write service tests (use mocks)**

```go
// internal/service/message_service_test.go
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
)

type mockMessageRepo struct {
	mock.Mock
}

func (m *mockMessageRepo) FindBySyncID(ctx context.Context, syncID string) (*model.StoredMessage, error) {
	args := m.Called(ctx, syncID)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*model.StoredMessage), args.Error(1)
}
// ... implement other methods with stubs returning zero values

func TestCreateServerMessage(t *testing.T) {
	repo := new(mockMessageRepo)
	svc := NewMessageService(repo, nil, nil, nil, nil, nil)
	repo.On("CreateServerMessage", mock.Anything, "alice", "Hello").Return(&model.StoredMessage{
		SyncID: "abc", Author: "alice", Content: "Hello",
	}, nil)

	msg, err := svc.CreateServerMessage(context.Background(), "alice", "Hello")
	assert.NoError(t, err)
	assert.Equal(t, "alice", msg.Author)
	repo.AssertExpectations(t)
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Implement all service files**

Direct Go ports of the Kotlin service classes. All methods take `context.Context` as first parameter.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat: add service layer — message, contact, security, email, outbox, analytics"
```

---

## Task 8: pkg/ Framework Libraries (i18n, theme, plugin)

**Files:**
- Create: `pkg/i18n/i18n.go`
- Create: `pkg/i18n/language.go`
- Create: `pkg/i18n/i18n_test.go`
- Create: `pkg/theme/theme.go`
- Create: `pkg/theme/shader.go`
- Create: `pkg/theme/theme_test.go`
- Create: `pkg/plugin/plugin.go`
- Create: `pkg/plugin/manager.go`
- Create: `pkg/plugin/manager_test.go`

- [ ] **Step 1: Port outerstellar-i18n to Go**

Replace `ResourceBundle` with Go's `embed.FS` + `.properties` parser. Replace `CopyOnWriteArrayList` with `sync.RWMutex` + slice. The `Translatable` interface is just `UpdateTexts() error`.

- [ ] **Step 2: Port outerstellar-theme to Go**

Replace Jackson with `encoding/json`. The `SmartShader` color math is pure arithmetic — direct port. Use `embed.FS` for bundled `themes.json`.

- [ ] **Step 3: Port outerstellar-plugin to Go**

Replace `ServiceLoader` with explicit `Register(name, factory)` pattern via `init()` functions. The `PluginManager` becomes a `sync.Map` based registry.

- [ ] **Step 4: Write and run tests for each package**

Run: `go test ./pkg/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/
git commit -m "feat: add pkg/ framework libraries — i18n, theme, plugin"
```

---

## Task 9: Web Layer — Filters + Middleware

**Files:**
- Create: `internal/web/filter/cors.go`
- Create: `internal/web/filter/csrf.go`
- Create: `internal/web/filter/security_headers.go`
- Create: `internal/web/filter/session.go`
- Create: `internal/web/filter/auth.go`
- Create: `internal/web/filter/logging.go`
- Create: `internal/web/filter/telemetry.go`
- Create: `internal/web/filter/rate_limiter.go`
- Create: `internal/web/filter/etag.go`
- Create: `internal/web/filter/metrics.go`
- Create: `internal/web/filter/error_handler.go`
- Create: `internal/web/context.go`
- Create: `internal/web/session_cookie.go`

- [ ] **Step 1: Implement WebContext**

```go
// internal/web/context.go
package web

import (
	"context"
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/pkg/i18n"
)

type contextKey string

const webContextKey contextKey = "webContext"

type WebContext struct {
	User      *model.User
	Theme     string
	Language  string
	Layout    string
	IsDark    bool
	CSRFToken string
	Version   string
	I18n      *i18n.I18nService
	Request   *http.Request
}

func (ctx *WebContext) URL(path string) string {
	scheme := "http"
	if ctx.Request.TLS != nil || ctx.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request.Host + path
}

func WebContextFromRequest(r *http.Request) *WebContext {
	ctx, _ := r.Context().Value(webContextKey).(*WebContext)
	return ctx
}

func WithWebContext(r *http.Request, wctx *WebContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), webContextKey, wctx))
}
```

- [ ] **Step 2: Implement all chi middleware**

Each filter is a `func(http.Handler) http.Handler` middleware. Port each Kotlin filter from `Filters.kt` to Go. The auth filter uses the `AuthRealm` chain.

- [ ] **Step 3: Implement session cookie helpers**

```go
// internal/web/session_cookie.go
package web

import "net/http"

func CreateSessionCookie(userID string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name: "oss_session", Value: userID,
		Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: 0,
	}
}

func ClearSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name: "oss_session", Value: "",
		Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/web/filter/ internal/web/context.go internal/web/session_cookie.go
git commit -m "feat: add chi middleware — CORS, CSRF, auth, session, rate limiting, metrics"
```

---

## Task 10: Web Layer — Templates + Renderer

**Files:**
- Create: `internal/web/renderer.go`
- Create: `internal/web/viewmodel/shell.go`
- Create: `internal/web/viewmodel/page.go`
- Create: `internal/web/viewmodel/viewmodels.go`
- Create: `internal/web/theme_catalog.go`
- Create: all `.html` template files under `internal/web/template/`

- [ ] **Step 1: Create template renderer**

```go
// internal/web/renderer.go
package web

import (
	"html/template"
	"io/fs"
	"net/http"
)

type Renderer struct {
	templates *template.Template
}

func NewRenderer(templateFS fs.FS, funcs template.FuncMap) (*Renderer, error) {
	tmpl := template.New("").Funcs(funcs)
	tmpl, err := tmpl.ParseFS(templateFS, "**/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tmpl}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.templates.ExecuteTemplate(w, name, data)
}
```

- [ ] **Step 2: Convert JTE templates to Go html/template**

Convert each `.kte` file to `.html` with Go template syntax:
- `@param` → define struct and pass as data
- `@if` / `@else` / `@endif` → `{{ if }}` / `{{ else }}` / `{{ end }}`
- `@for` → `{{ range }}`
- `${var}` → `{{ .Var }}`
- `@template.xxx()` → `{{ template "xxx" . }}`
- `!{var}` (unsafe) → `{{ .Var | safeHTML }}`

- [ ] **Step 3: Create view models**

Port all Kotlin data classes from `ViewModels.kt` and `WebPageFactory.kt` as Go structs with JSON/template tags.

- [ ] **Step 4: Commit**

```bash
git add internal/web/renderer.go internal/web/viewmodel/ internal/web/template/ internal/web/theme_catalog.go
git commit -m "feat: add HTML template renderer, view models, and converted JTE templates"
```

---

## Task 11: Web Layer — HTTP Handlers

**Files:**
- Create: `internal/web/handler/sync_api.go`
- Create: `internal/web/handler/auth_api.go`
- Create: `internal/web/handler/auth.go`
- Create: `internal/web/handler/home.go`
- Create: `internal/web/handler/contacts.go`
- Create: `internal/web/handler/user_admin.go`
- Create: `internal/web/handler/user_admin_api.go`
- Create: `internal/web/handler/notifications.go`
- Create: `internal/web/handler/notification_api.go`
- Create: `internal/web/handler/device_registration_api.go`
- Create: `internal/web/handler/oauth.go`
- Create: `internal/web/handler/search.go`
- Create: `internal/web/handler/settings.go`
- Create: `internal/web/handler/errors.go`
- Create: `internal/web/handler/dev_dashboard.go`
- Create: `internal/web/handler/components.go`
- Create: `internal/web/handler/sync_websocket.go`

- [ ] **Step 1: Implement Sync API handler**

Port `SyncApi.kt` to Go. The sync endpoints are critical for the desktop app:

```go
// internal/web/handler/sync_api.go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
)

type SyncAPI struct {
	messageService *service.MessageService
	contactService *service.ContactService
	analytics      service.AnalyticsService
}

func (h *SyncAPI) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/sync", h.PullMessages)
	r.Post("/api/v1/sync", h.PushMessages)
	r.Get("/api/v1/sync/contacts", h.PullContacts)
	r.Post("/api/v1/sync/contacts", h.PushContacts)
}

func (h *SyncAPI) PullMessages(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	var since int64
	if sinceStr != "" {
		since, _ = strconv.ParseInt(sinceStr, 10, 64)
	}
	resp := h.messageService.GetChangesSince(r.Context(), since)
	writeJSON(w, http.StatusOK, resp)
}

func (h *SyncAPI) PushMessages(w http.ResponseWriter, r *http.Request) {
	var req model.SyncPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp := h.messageService.ProcessPushRequest(r.Context(), req)
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Implement Auth API handler**

Port `AuthApi.kt` — login, register, password reset, API keys, profile management.

- [ ] **Step 3: Implement Auth UI handler**

Port `AuthRoutes.kt` — form-based login/register/change-password pages.

- [ ] **Step 4: Implement remaining handlers**

Port each Kotlin route handler to Go. Each is a struct with methods on `chi.Router`.

- [ ] **Step 5: Write handler tests**

```go
// internal/web/handler/sync_api_test.go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/rygel/gouterstellar-platform/internal/model"
)

func TestPullMessages(t *testing.T) {
	// Setup with mock services
	api := &SyncAPI{messageService: mockMsgSvc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync?since=0", nil)
	w := httptest.NewRecorder()
	api.PullMessages(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/web/handler/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/web/handler/
git commit -m "feat: add HTTP handlers for sync, auth, contacts, admin, notifications"
```

---

## Task 12: Application Wiring + main.go

**Files:**
- Create: `internal/wire/wire.go`
- Create: `cmd/server/main.go`
- Create: `cmd/seed/main.go`

- [ ] **Step 1: Implement manual DI wiring**

```go
// internal/wire/wire.go
package wire

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web/handler"
)

type App struct {
	Config          *config.Config
	MessageService  *service.MessageService
	ContactService  *service.ContactService
	SecurityService *service.SecurityService
	SyncAPI         *handler.SyncAPI
	AuthAPI         *handler.AuthAPI
	Home            *handler.HomeHandler
	// ... all wired components
}

func Wire(cfg *config.Config, pool *pgxpool.Pool) *App {
	// Repositories
	msgRepo := persistence.NewMessageRepository(pool)
	contactRepo := persistence.NewContactRepository(pool)
	userRepo := persistence.NewUserRepository(pool)
	sessionRepo := persistence.NewSessionRepository(pool)
	apiKeyRepo := persistence.NewApiKeyRepository(pool)
	outboxRepo := persistence.NewOutboxRepository(pool)
	auditRepo := persistence.NewAuditRepository(pool)
	notifRepo := persistence.NewNotificationRepository(pool)

	// Cache
	cache := persistence.NewMessageCache(10 * time.Minute)

	// Security
	passwordEncoder := security.NewBCryptPasswordEncoder(12)
	jwtSvc := security.NewJwtService(cfg.JWT)
	activityUpdater := security.NewAsyncActivityUpdater(userRepo)
	permissionResolver := security.NewRoleBasedPermissionResolver()
	realms := []security.AuthRealm{
		security.NewSessionRealm(securitySvc, sessionRepo),
		security.NewApiKeyRealm(securitySvc, apiKeyRepo),
	}

	// Services
	securitySvc := service.NewSecurityService(userRepo, passwordEncoder, auditRepo, ...)
	msgSvc := service.NewMessageService(msgRepo, outboxRepo, tm, cache, eventPub, auditRepo)
	contactSvc := service.NewContactService(contactRepo, eventPub, tm, auditRepo)
	outboxProcessor := service.NewOutboxProcessor(outboxRepo, tm)

	// Handlers
	syncAPI := handler.NewSyncAPI(msgSvc, contactSvc, analytics)
	authAPI := handler.NewAuthAPI(securitySvc)
	home := handler.NewHomeHandler(msgSvc, renderer, pageFactory)

	return &App{ /* ... */ }
}
```

- [ ] **Step 2: Implement main.go**

```go
// cmd/server/main.go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	"github.com/rygel/gouterstellar-platform/internal/wire"
)

func main() {
	cfg := config.Load()
	slog.Info("Starting Outerstellar Platform", "version", cfg.Version, "port", cfg.Port)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Run migrations
	// migrate.Up(cfg.DatabaseURL, "migrations")

	app := wire.Wire(cfg, pool)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(filter.CORS(cfg.CORSOrigins))
	r.Use(filter.SecurityHeaders(cfg.CSPPolicy))
	r.Use(filter.RateLimit())
	r.Use(filter.CSRF(cfg.SessionCookieSecure, cfg.CSRFEnabled))
	r.Use(filter.Session(cfg.SessionTimeoutMinutes, pool, cfg.SessionCookieSecure))
	r.Use(filter.Auth(app.Realms))
	r.Use(filter.RequestLogging())

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	r.Handle("/vendor/*", http.StripPrefix("/vendor/", http.FileServer(http.Dir("static/vendor"))))
	r.Handle("/site.css", http.FileServer(http.Dir("static")))
	r.Handle("/platform.js", http.FileServer(http.Dir("static")))

	// Health
	r.Get("/health", app.HealthHandler)
	r.Get("/metrics", app.MetricsHandler)

	// API routes (bearer auth)
	r.Route("/api", func(r chi.Router) {
		r.Use(filter.BearerAuth(app.Realms))
		app.SyncAPI.RegisterRoutes(r)
		app.AuthAPI.RegisterRoutes(r)
		app.UserAdminAPI.RegisterRoutes(r)
		app.NotificationAPI.RegisterRoutes(r)
		app.DeviceRegistrationAPI.RegisterRoutes(r)
	})

	// UI routes (session auth via middleware)
	app.Auth.RegisterRoutes(r)
	app.Home.RegisterRoutes(r)
	app.Contacts.RegisterRoutes(r)
	app.UserAdmin.RegisterRoutes(r)
	app.Notifications.RegisterRoutes(r)
	app.Search.RegisterRoutes(r)
	app.Settings.RegisterRoutes(r)
	app.OAuth.RegisterRoutes(r)
	app.Errors.RegisterRoutes(r)
	app.Components.RegisterRoutes(r)

	// Admin routes (session + role check)
	r.Route("/admin", func(r chi.Router) {
		r.Use(filter.RequireRole(model.RoleAdmin))
		app.DevDashboard.RegisterRoutes(r)
	})

	// Start server
	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
```

- [ ] **Step 3: Build and verify**

Run: `go build -o bin/server ./cmd/server`
Expected: Builds successfully

- [ ] **Step 4: Commit**

```bash
git add internal/wire/ cmd/
git commit -m "feat: add application wiring and server entry point"
```

---

## Task 13: Seed Data + Static Assets

**Files:**
- Create: `cmd/seed/main.go`
- Copy: `static/` (site.css, platform.js, vendor/ from Kotlin repo)

- [ ] **Step 1: Port seed data**

```go
// cmd/seed/main.go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/security"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	encoder := security.NewBCryptPasswordEncoder(12)
	userRepo := persistence.NewUserRepository(pool)
	hash, _ := encoder.Encode("admin")
	userRepo.SeedAdminUser(ctx, hash)
	slog.Info("Seed data inserted")
}
```

- [ ] **Step 2: Copy static assets from Kotlin repo**

Copy `platform-web/src/main/resources/static/` to `static/`.

- [ ] **Step 3: Commit**

```bash
git add cmd/seed/ static/
git commit -m "feat: add seed data command and static web assets"
```

---

## Task 14: End-to-End Verification

**Files:**
- Create: `internal/web/handler/e2e_test.go`

- [ ] **Step 1: Start the server and test the full auth + sync flow**

```go
// internal/web/handler/e2e_test.go
package handler

import (
	"testing"
	"net/http/httptest"
	// ...
)

func TestE2E_RegisterLoginSync(t *testing.T) {
	// 1. POST /api/v1/auth/register → get token
	// 2. POST /api/v1/sync with Bearer token → push messages
	// 3. GET /api/v1/sync?since=0 → pull messages
	// 4. Verify messages round-trip
}
```

- [ ] **Step 2: Test the desktop app contract**

Verify the Go server returns the same JSON shapes the Kotlin server does:
- `POST /api/v1/auth/login` → `{"token":"oss_...","username":"...","role":"USER"}`
- `GET /api/v1/sync?since=0` → `{"messages":[...],"serverTimestamp":...}`
- `POST /api/v1/sync` → `{"appliedCount":1,"conflicts":[]}`

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -timeout 120s`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/web/handler/e2e_test.go
git commit -m "test: add end-to-end verification for auth + sync contract"
```

---

## Task Dependency Graph

```
Task 1 (scaffold) ──► Task 2 (models) ──► Task 3 (config)
         │                                      │
         ▼                                      ▼
    Task 4 (sqlc + repos) ──► Task 5 (repo impls) ──► Task 6 (security)
                                                          │
                      Task 7 (services) ◄─────────────────┘
                          │
                          ▼
          ┌─── Task 8 (pkg/ i18n/theme/plugin)
          │
          ├─── Task 9 (filters)
          │
          ├─── Task 10 (templates)
          │        │
          │        ▼
          └─── Task 11 (handlers) ──► Task 12 (wiring + main)
                                        │
                                        ▼
                                  Task 13 (seed + assets)
                                        │
                                        ▼
                                  Task 14 (E2E verification)
```

Tasks 8, 9, 10 can be done in parallel. Task 5 requires a running PostgreSQL (or testcontainers). The critical path is Tasks 1 → 2 → 4 → 5 → 6 → 7 → 11 → 12.

---

## Desktop App Compatibility Contract

The Go server must expose these exact endpoints with identical JSON shapes:

| Endpoint | Method | Used by Desktop |
|----------|--------|----------------|
| `/api/v1/auth/login` | POST | SyncService.login() |
| `/api/v1/auth/register` | POST | SyncService.register() |
| `/api/v1/auth/password` | PUT | SyncService.changePassword() |
| `/api/v1/auth/reset-request` | POST | SyncService.requestPasswordReset() |
| `/api/v1/auth/reset-confirm` | POST | SyncService.resetPassword() |
| `/api/v1/auth/profile` | GET/PUT | SyncService.fetchProfile()/updateProfile() |
| `/api/v1/auth/notification-preferences` | PUT | SyncService.updateNotificationPreferences() |
| `/api/v1/auth/account` | DELETE | SyncService.deleteAccount() |
| `/api/v1/auth/api-keys` | POST/GET | SyncService.createApiKey()/listApiKeys() |
| `/api/v1/auth/api-keys/{id}` | DELETE | SyncService.deleteApiKey() |
| `/api/v1/sync` | GET | SyncService.sync() (pull) |
| `/api/v1/sync` | POST | SyncService.sync() (push) |
| `/api/v1/sync/contacts` | GET | Contact sync pull |
| `/api/v1/sync/contacts` | POST | Contact sync push |
| `/api/v1/notifications` | GET | SyncService.listNotifications() |
| `/api/v1/notifications/{id}/read` | PUT | SyncService.markNotificationRead() |
| `/api/v1/notifications/read-all` | PUT | SyncService.markAllNotificationsRead() |
| `/api/v1/admin/users` | GET | SyncService.listUsers() |
| `/api/v1/admin/users/{id}/enabled` | PUT | SyncService.setUserEnabled() |
| `/api/v1/admin/users/{id}/role` | PUT | SyncService.setUserRole() |
| `/health` | GET | Connectivity check |
