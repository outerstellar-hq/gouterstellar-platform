# Private Platform Features — Go Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the 9 missing features from outerstellar-platform-private into the Go port.

**Architecture:** Add features incrementally with backward compatibility. Each task is self-contained and can be committed independently. Follow existing patterns: chi routes, sqlc queries, manual DI in wire.go, gohtml templates.

**Tech Stack:** gorilla/websocket (new dep), existing chi/pgx/viper/sqlc stack.

---

### Task 1: JWT Auth Realm + Token Issuance Endpoint

Wire the existing (but dormant) JwtService into the auth pipeline and add a token issuance endpoint.

**Files:**
- Modify: `internal/security/auth_realm.go`
- Modify: `internal/web/handler/auth_api.go`
- Modify: `internal/wire/wire.go`
- Modify: `internal/web/handler/auth_api.go` (routes)

- [ ] **Step 1: Add JwtRealm to auth_realm.go**

Add a `jwtRealm` that validates JWT tokens via JwtService and looks up the user:

```go
type JwtLookupFunc func(userID uuid.UUID) *model.User

type jwtRealm struct {
	jwtSvc   *JwtService
	userRepo JwtLookupFunc
}

func NewJwtRealm(jwtSvc *JwtService, lookup JwtLookupFunc) AuthRealm {
	return &jwtRealm{jwtSvc: jwtSvc, userRepo: lookup}
}

func (r *jwtRealm) Name() string { return "jwt" }

func (r *jwtRealm) Authenticate(token string) AuthResult {
	if !r.jwtSvc.IsEnabled() {
		return SkippedResult{}
	}
	userID, _, err := r.jwtSvc.ExtractClaims(token)
	if err != nil {
		return SkippedResult{}
	}
	user := r.userRepo(userID)
	if user == nil || !user.Enabled {
		return SkippedResult{}
	}
	return AuthenticatedResult{User: user}
}
```

- [ ] **Step 2: Add JWT token issuance endpoint to auth_api.go**

Add `POST /api/v1/auth/token` that accepts username/password and returns a JWT (not a session). Add `jwtService` field to `AuthAPI` struct. The handler reuses the existing `Authenticate` + `jwtSvc.GenerateToken` flow. Returns `{"token": "...", "expires_in": 3600}`.

- [ ] **Step 3: Wire JwtRealm in wire.go**

After `apiKeyRealm`, create `jwtRealm` and add it to the `realms` slice. Pass `jwtSvc` to `NewAuthAPI`.

- [ ] **Step 4: Build and verify**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 5: Commit**

```
feat: wire JWT auth realm and add token issuance endpoint
```

---

### Task 2: Gravatar Integration

Auto-generate Gravatar avatar URLs from email when no avatar_url is set.

**Files:**
- Create: `internal/web/gravatar.go`
- Modify: `internal/web/handler/auth_api.go` (GetProfile response)

- [ ] **Step 1: Create gravatar.go**

```go
package web

import (
	"crypto/md5"
	"fmt"
	"strings"
)

func GravatarURL(email string, size int) string {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("https://www.gravatar.com/avatar/%x?d=identicon&s=%d", hash, size)
}
```

- [ ] **Step 2: Use Gravatar in GetProfile**

In `auth_api.go` `GetProfile`, if `user.AvatarURL` is nil, compute Gravatar from `user.Email`:

```go
avatarURL := ""
if user.AvatarURL != nil {
    avatarURL = *user.AvatarURL
} else if user.Email != "" {
    avatarURL = web.GravatarURL(user.Email, 80)
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```
feat: add Gravatar integration for user avatars
```

---

### Task 3: CSV Export for Admin (Users + Audit Log)

Add CSV download endpoints for admin users and audit log.

**Files:**
- Create: `internal/web/handler/csv_export.go`
- Modify: `internal/web/handler/user_admin.go` (routes + export handlers)
- Modify: `internal/web/handler/user_admin_api.go` (routes + export handlers)

- [ ] **Step 1: Create csv_export.go helper**

```go
package handler

import (
	"encoding/csv"
	"net/http"
)

func writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	writer := csv.NewWriter(w)
	_ = writer.Write(headers)
	for _, row := range rows {
		_ = writer.Write(row)
	}
	writer.Flush()
}
```

- [ ] **Step 2: Add ExportUsers to user_admin.go**

Add route `r.Get("/admin/users/export", h.ExportUsers)`. Handler fetches all users via `CountUsers` + `ListUsersPaged` with a large page size, converts to CSV rows (username, email, role, enabled, created_at).

- [ ] **Step 3: Add ExportAudit to user_admin.go**

Add route `r.Get("/admin/audit/export", h.ExportAudit)`. Handler fetches audit entries and writes CSV (actor, target, action, detail, created_at).

- [ ] **Step 4: Add JSON CSV export routes to user_admin_api.go**

Add `GET /api/v1/admin/users/export` and `GET /api/v1/admin/audit/export` that return the same CSV with proper content-type headers.

- [ ] **Step 5: Build and verify**

Run: `go build ./...`

- [ ] **Step 6: Commit**

```
feat: add CSV export for admin users and audit log
```

---

### Task 4: Health Check with DB Connectivity

Enhance the `/health` endpoint to verify database connectivity.

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Replace simple health check with DB-aware check**

Change the inline health handler to accept a `pgxpool.Pool` and check connectivity:

```go
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()
    if err := pool.Ping(ctx); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        _, _ = fmt.Fprintf(w, `{"status":"unhealthy","database":"down","error":%q}`, err.Error())
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = fmt.Fprintf(w, `{"status":"healthy","database":"up"}`)
})
```

The `pool` variable is already in scope in `main()`.

- [ ] **Step 2: Build and verify**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```
feat: enhance health check with database connectivity test
```

---

### Task 5: Developer Auto-Login

In dev mode, automatically log in as admin on local connections.

**Files:**
- Create: `internal/web/filter/dev_autologin.go`
- Modify: `cmd/server/main.go` (add filter)

- [ ] **Step 1: Create dev_autologin.go**

```go
package filter

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

func DevAutoLogin(secSvc *service.SecurityService, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if web.UserFromRequest(r) != nil {
				next.ServeHTTP(w, r)
				return
			}
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			if !net.ParseIP(host).IsLoopback() {
				next.ServeHTTP(w, r)
				return
			}
			token, err := secSvc.CreateSession(r.Context(), secSvc.DevAdminID(r.Context()))
			if err != nil {
				slog.Warn("Dev auto-login failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			cookie := web.CreateSessionCookie(token, false)
			http.SetCookie(w, cookie)
			r.AddCookie(cookie)
			slog.Debug("Dev auto-login", "host", host)
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: Add DevAdminID method to SecurityService**

In `security_service.go`, add a method that looks up the admin user ID:

```go
func (s *SecurityService) DevAdminID(ctx context.Context) uuid.UUID {
	users, _ := s.ListUsersPaged(ctx, 1, 0)
	for _, u := range users {
		if u.Role == model.RoleAdmin {
			return u.ID
		}
	}
	return uuid.Nil
}
```

- [ ] **Step 3: Wire in main.go**

Add the dev auto-login filter before the session filter, gated by `cfg.DevMode`:

```go
r.Use(filter.DevAutoLogin(app.SecurityService, cfg.DevMode))
```

- [ ] **Step 4: Build and verify**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```
feat: add developer auto-login for dev mode
```

---

### Task 6: Real-Time Sync WebSocket

Add a WebSocket endpoint at `/ws/sync` that broadcasts sync events to connected clients.

**Files:**
- Create: `internal/web/handler/sync_ws.go`
- Create: `internal/service/ws_event_publisher.go`
- Modify: `internal/wire/wire.go` (replace NoOpEventPublisher)
- Modify: `cmd/server/main.go` (register route)

- [ ] **Step 1: Add gorilla/websocket dependency**

Run: `go get github.com/gorilla/websocket@latest`

- [ ] **Step 2: Create ws_event_publisher.go**

A thread-safe hub that manages WebSocket connections and broadcasts:

```go
package service

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

type WsClient struct {
	UserID string
	Conn   *websocket.Conn
}

type WsEventPublisher struct {
	mu      sync.RWMutex
	clients map[*WsClient]struct{}
}

func NewWsEventPublisher() *WsEventPublisher {
	return &WsEventPublisher{clients: make(map[*WsClient]struct{})}
}

func (p *WsEventPublisher) Register(client *WsClient) {
	p.mu.Lock()
	p.clients[client] = struct{}{}
	p.mu.Unlock()
}

func (p *WsEventPublisher) Unregister(client *WsClient) {
	p.mu.Lock()
	delete(p.clients, client)
	p.mu.Unlock()
}

func (p *WsEventPublisher) PublishRefresh(targetID string) {
	msg := "refresh:" + targetID
	p.mu.RLock()
	defer p.mu.RUnlock()
	for client := range p.clients {
		err := client.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			slog.Warn("WS write failed", "userID", client.UserID, "error", err)
		}
	}
}
```

- [ ] **Step 3: Create sync_ws.go handler**

WebSocket upgrade handler that validates session cookie, registers the client, and reads from the connection (ping/pong keepalive). Extract user from session cookie using the session realm lookup pattern.

```go
package handler

type SyncWebSocket struct {
	publisher  *service.WsEventPublisher
	secSvc     *service.SecurityService
	sessionSec bool
}

func NewSyncWebSocket(publisher *service.WsEventPublisher, secSvc *service.SecurityService, sessionSec bool) *SyncWebSocket {
	return &SyncWebSocket{publisher: publisher, secSvc: secSvc, sessionSec: sessionSec}
}

func (h *SyncWebSocket) RegisterRoutes(r chi.Router) {
	r.Get("/ws/sync", h.Handle)
}

func (h *SyncWebSocket) Handle(w http.ResponseWriter, r *http.Request) {
	// Validate session cookie -> get user
	// Upgrade to WebSocket
	// Register client
	// Read loop (discard messages, detect close)
	// On disconnect: unregister client
}
```

Use `gorilla/websocket.Upgrader` with `CheckOrigin` that allows configured CORS origins.

- [ ] **Step 4: Replace NoOpEventPublisher in wire.go**

Replace `eventPub := &service.NoOpEventPublisher{}` with `eventPub := service.NewWsEventPublisher()`.

Pass `eventPub` (as `*WsEventPublisher`) to the `SyncWebSocket` handler and store in `App`.

- [ ] **Step 5: Register route in main.go**

Add `app.SyncWebSocket.RegisterRoutes(r)` in the appropriate route group. This route must be outside the BearerAuth group since it uses session cookies.

- [ ] **Step 6: Build and verify**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```
feat: add real-time sync WebSocket endpoint
```

---

### Task 7: Settings Page Tabs (Password, Notifications, Appearance)

Upgrade the settings page from flat sections to a tabbed interface matching the Kotlin version.

**Files:**
- Modify: `internal/web/template/pages/settings.html`
- Modify: `internal/web/handler/settings.go`
- Modify: `internal/web/viewmodel/page.go` (if needed)

- [ ] **Step 1: Update SettingsPage viewmodel**

Add fields for active tab, API keys list, password form:

```go
type SettingsPage struct {
	ActiveTab string
	Profile   ProfileData
	Theme     string
	Language  string
	Layout    string
	ApiKeys   []ApiKeyItem
}
```

- [ ] **Step 2: Rewrite settings.html with tabs**

Create a tabbed interface with 5 tabs:
1. **Profile** — username, email, avatar, notification toggles
2. **Password** — current + new password form
3. **API Keys** — list + create form
4. **Notifications** — email/push toggle checkboxes
5. **Appearance** — theme selector, language, layout (sidebar/topbar, density)

Use query param `?tab=profile` to control active tab. Add CSS for tab navigation.

- [ ] **Step 3: Update settings handler to load API keys and active tab**

In `Show`, read `?tab=` query param (default "profile"), fetch user's API keys, populate the viewmodel with layout field.

- [ ] **Step 4: Add password change handler to settings**

Add route `POST /settings/password` that calls `securityService.ChangePassword`.

- [ ] **Step 5: Add notification preferences handler to settings**

Add route `POST /settings/notifications` that reads checkbox values and calls `securityService.UpdateNotificationPreferences`.

- [ ] **Step 6: Build and verify**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```
feat: upgrade settings page to tabbed interface with password, notifications, and appearance
```

---

### Task 8: UI Layout System (Sidebar/Topbar Toggle + Density)

Add layout selection UI that persists the user's layout preference.

**Files:**
- Modify: `internal/web/template/pages/settings.html` (appearance tab)
- Modify: `internal/web/template/partials/shell.html` (or base layout template)
- Modify: `static/css/app.css`

- [ ] **Step 1: Add layout options to settings appearance tab**

In the appearance tab, add:
- Shell layout: "Sidebar" / "Topbar" radio buttons
- Density: "Compact" / "Cozy" / "Spacious" radio buttons

These are already persisted via the `layout` column (e.g., `"sidebar-compact"`, `"topbar-cozy"`). The `UpdatePreferences` handler already reads the `layout` form field.

- [ ] **Step 2: Add CSS classes for layout modes**

Add CSS classes `.layout-sidebar`, `.layout-topbar`, `.density-compact`, `.density-cozy`, `.density-spacious` that adjust the shell template. The shell template should read the user's layout preference and apply the appropriate class to `<body>`.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```
feat: add UI layout system with sidebar/topbar and density options
```

---

### Task 9: Plugin Navigation Items

Add navigation item support to the plugin system and wire it into the shell.

**Files:**
- Modify: `pkg/plugin/plugin.go`
- Modify: `pkg/plugin/manager.go`
- Modify: `internal/web/viewmodel/shell.go`
- Modify: `internal/wire/wire.go`

- [ ] **Step 1: Add NavItems to ServerPlugin interface**

In `pkg/plugin/plugin.go`, add:

```go
type PluginNavItem struct {
	Label    string
	URL      string
	Icon     string
	Children []PluginNavItem
}

type ServerPlugin interface {
	Plugin
	NavItems() []PluginNavItem
}
```

- [ ] **Step 2: Add method to PluginManager**

In `manager.go`, add:

```go
func (m *PluginManager) AllNavItems() []PluginNavItem {
	var items []PluginNavItem
	for _, p := range m.plugins {
		if sp, ok := p.(ServerPlugin); ok {
			items = append(items, sp.NavItems()...)
		}
	}
	return items
}
```

- [ ] **Step 3: Wire plugin manager into App**

In `wire.go`, create `PluginManager` and store in `App`. Convert `PluginNavItem` to `viewmodel.NavItem` when populating the shell.

- [ ] **Step 4: Populate ShellViewModel.NavItems from plugins**

In the renderer or shell template execution, check if the plugin manager has nav items and merge them into `ShellViewModel.NavItems`. If plugins provide nav items, they replace the default navigation.

- [ ] **Step 5: Build and verify**

Run: `go build ./...`

- [ ] **Step 6: Commit**

```
feat: add plugin navigation items support
```

---

## Dependency Order

Tasks can be done in any order except:
- Task 7 (Settings tabs) should be done before Task 8 (Layout) since Task 8 modifies the appearance tab added in Task 7
- Task 6 (WebSocket) should be done before Task 9 (Plugin NavItems) since they're independent but the WebSocket is higher priority

Recommended order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9

## Verification

After all tasks are complete:
1. `go build ./...` — clean build
2. `go vet ./...` — clean
3. `golangci-lint run ./...` — clean
4. `gosec -exclude-dir=internal/persistence/db ./...` — clean
5. `gofumpt -l .` — clean
6. `goimports -l -local github.com/rygel/gouterstellar-platform .` — clean
7. `go test ./... -timeout 120s -count=1` — all pass
