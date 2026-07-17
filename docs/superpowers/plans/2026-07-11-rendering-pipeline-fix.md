# Rendering Pipeline Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the fundamentally broken rendering pipeline so every HTML route renders correctly through a shared layout with ShellViewModel.

**Architecture:** Per-page clone pattern for Go template inheritance. Renderer maintains `map[string]*template.Template` (one clone per page). `RenderPage` builds ShellViewModel from request context automatically. `RenderPartial` for HTMX fragment responses.

**Tech Stack:** Go stdlib `html/template`, `embed.FS`, testify

**Spec:** `docs/superpowers/specs/2026-07-11-rendering-pipeline-fix-design.md`

---

## File Structure

### New files

| Path | Responsibility |
|---|---|
| `internal/web/template/pages/auth_login.html` | Login + register form (renamed from auth.html) |
| `internal/web/template/pages/auth_change_password.html` | Change password form |
| `internal/web/template/pages/auth_reset_password.html` | Password reset request form |
| `internal/web/template/pages/auth_reset_sent.html` | Reset email sent confirmation |
| `internal/web/template/pages/search.html` | Search results page |
| `internal/web/template/pages/dev_dashboard.html` | Dev ops dashboard |
| `internal/web/template/partials/message_list.html` | Message list fragment for HTMX |
| `internal/web/template/partials/contact_list.html` | Contact list fragment for HTMX |
| `internal/web/handler/messages.go` | Messages page handler (GET /messages) |
| `internal/web/renderer_test.go` | Renderer unit tests |

### Modified files

| Path | Change |
|---|---|
| `internal/web/renderer.go` | Per-page clone parsing, RenderPage, RenderPartial, buildShell |
| `internal/web/template/base.html` | Add CSRF meta tag, script tag for platform.js |
| `internal/web/handler/home.go` | Render call → RenderPage |
| `internal/web/handler/contacts.go` | Render calls → RenderPage |
| `internal/web/handler/notifications.go` | Render calls → RenderPage |
| `internal/web/handler/settings.go` | Render calls → RenderPage |
| `internal/web/handler/user_admin.go` | Render calls → RenderPage |
| `internal/web/handler/auth.go` | Render calls → RenderPage, fix template names |
| `internal/web/handler/search.go` | Render calls → RenderPage |
| `internal/web/handler/errors.go` | Render calls → RenderPage |
| `internal/web/handler/components.go` | Render calls → RenderPartial |
| `internal/web/handler/dev_dashboard.go` | Render calls → RenderPage |
| `internal/wire/wire.go` | Add MessagesHandler, version param to NewRenderer |
| `internal/platform/core/core.go` | Add MessagesShow to Bundle |
| `internal/platform/core/contribute.go` | Add /messages route + nav |
| `cmd/server/main.go` | Pass version to NewRenderer |

### Deleted files

| Path | Reason |
|---|---|
| `internal/web/template/pages/auth.html` | Renamed to auth_login.html |
| `pkg/theme/theme.go` | Dead code — zero imports |
| `pkg/theme/shader.go` | Dead code |
| `pkg/theme/theme_test.go` | Dead code |
| `internal/model/theme.go` | Dead code — ThemeDefinition never referenced |

---

## Task 1: Rewrite the Renderer

**Files:**
- Modify: `internal/web/renderer.go`
- Test: `internal/web/renderer_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/web/renderer_test.go
package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"template/base.html": &fstest.MapFile{
			Data: []byte(`{{ define "base" }}<html><head><title>{{ .Title }}</title></head><body><nav>NAV</nav><main>{{ template "content" .BodyData }}</main></body></html>{{ end }}`),
		},
		"template/partials/pagination.html": &fstest.MapFile{
			Data: []byte(`{{ define "pagination" }}<div class="pagination">page {{ .CurrentPage }}</div>{{ end }}`),
		},
		"template/pages/home.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>{{ .Title }}</h1>{{ end }}`),
		},
	}
}

func TestNewRendererParsesAllPages(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	assert.Contains(t, r.pages, "home", "home page should be parsed")
}

func TestRenderPageProducesShellAndContent(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err = r.RenderPage(rec, req, "home", map[string]string{"Title": "Welcome"})
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "<nav>NAV</nav>", "shell nav should be present")
	assert.Contains(t, body, "<h1>Welcome</h1>", "page content should be present")
}

func TestRenderPartialProducesFragment(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	err = r.RenderPartial(rec, "pagination", map[string]int{"CurrentPage": 3})
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "page 3")
	assert.NotContains(t, body, "<nav>", "partial should not contain shell chrome")
}

func TestRenderPageSetsContentType(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	_ = r.RenderPage(rec, req, "home", map[string]string{"Title": "X"})

	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web/ -run TestNew -v`
Expected: FAIL — `pages` field doesn't exist, `RenderPage`/`RenderPartial` undefined.

- [ ] **Step 3: Write the new Renderer**

```go
// internal/web/renderer.go
package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

// Renderer renders HTML pages through a shared layout (base.html) and
// partials for HTMX fragment responses. Each page is pre-cloned from the
// base template set so its {{ define "content" }} block resolves cleanly.
type Renderer struct {
	pages    map[string]*template.Template
	partials *template.Template
	version  string
}

// NewRenderer parses base.html + partials into a base set, then clones
// it for each page file. Pages are keyed by filename without extension.
func NewRenderer(templateFS fs.FS, funcs template.FuncMap, version string) (*Renderer, error) {
	// Parse base + partials into the shared base set.
	baseBytes, err := fs.ReadFile(templateFS, "template/base.html")
	if err != nil {
		return nil, fmt.Errorf("read base.html: %w", err)
	}

	base := template.New("").Funcs(funcs)
	base, err = base.Parse(string(baseBytes))
	if err != nil {
		return nil, fmt.Errorf("parse base.html: %w", err)
	}

	// Parse partials into the base set so they're available in every clone.
	partialEntries, err := fs.ReadDir(templateFS, "template/partials")
	if err != nil {
		return nil, fmt.Errorf("read partials dir: %w", err)
	}
	for _, entry := range partialEntries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		content, err := fs.ReadFile(templateFS, "template/partials/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read partial %s: %w", entry.Name(), err)
		}
		base, err = base.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse partial %s: %w", entry.Name(), err)
		}
	}

	// Clone for each page.
	pages := make(map[string]*template.Template)
	pageEntries, err := fs.ReadDir(templateFS, "template/pages")
	if err != nil {
		return nil, fmt.Errorf("read pages dir: %w", err)
	}
	for _, entry := range pageEntries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		pageName := strings.TrimSuffix(entry.Name(), ".html")
		content, err := fs.ReadFile(templateFS, "template/pages/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read page %s: %w", entry.Name(), err)
		}
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone for page %s: %w", pageName, err)
		}
		clone, err = clone.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse page %s: %w", pageName, err)
		}
		pages[pageName] = clone
	}

	// Separate partials set for fragment rendering.
	partials := template.New("").Funcs(funcs)
	for _, entry := range partialEntries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		content, _ := fs.ReadFile(templateFS, "template/partials/"+entry.Name())
		partials, err = partials.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse partials set %s: %w", entry.Name(), err)
		}
	}

	return &Renderer{pages: pages, partials: partials, version: version}, nil
}

// RenderPage renders a page wrapped in the shell layout.
// page is the page name without .html (e.g. "home", "contacts").
func (r *Renderer) RenderPage(w http.ResponseWriter, req *http.Request, page string, data interface{}) error {
	tmpl, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("unknown page template: %q", page)
	}

	shell := r.buildShell(req)
	shell.Body = page
	shell.BodyData = data
	shell.Title = pageTitle(page)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "base", shell)
}

// RenderWithStatus renders a page with a specific HTTP status code.
func (r *Renderer) RenderWithStatus(w http.ResponseWriter, req *http.Request, page string, data interface{}, status int) error {
	w.WriteHeader(status)
	return r.RenderPage(w, req, page, data)
}

// RenderPartial renders a fragment without shell wrapping (for HTMX responses).
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.partials.ExecuteTemplate(w, name, data)
}

// buildShell constructs a ShellViewModel from request context.
func (r *Renderer) buildShell(req *http.Request) *viewmodel.ShellViewModel {
	shell := &viewmodel.ShellViewModel{
		CSRFToken: CSRFTokenFromRequest(req),
		RequestID: RequestIDFromContext(req.Context()),
		Version:   r.version,
		Theme:     "light",
	}

	if user := UserFromRequest(req); user != nil {
		shell.User = &viewmodel.UserContext{
			ID:       user.ID.String(),
			Username: user.Username,
			Role:     string(user.Role),
			IsAdmin:  user.Role == model.RoleAdmin,
		}
		theme := "light"
		if user.Theme != nil && *user.Theme != "" {
			theme = *user.Theme
		}
		shell.Theme = theme
		shell.IsDark = theme == "dark"
		if user.Language != nil {
			shell.Language = *user.Language
		}
	}

	return shell
}

// pageTitle returns a human-readable title for a page name.
func pageTitle(page string) string {
	switch page {
	case "home":
		return "Dashboard"
	case "auth_login":
		return "Sign In"
	case "auth_change_password":
		return "Change Password"
	case "auth_reset_password":
		return "Reset Password"
	case "auth_reset_sent":
		return "Reset Sent"
	case "contacts":
		return "Contacts"
	case "messages":
		return "Messages"
	case "search":
		return "Search"
	case "settings":
		return "Settings"
	case "notifications":
		return "Notifications"
	case "admin_users":
		return "User Management"
	case "admin_audit":
		return "Audit Log"
	case "dev_dashboard":
		return "Dev Dashboard"
	case "error":
		return "Error"
	default:
		return strings.Title(page)
	}
}

// suppress unused import warning
var _ = path.Dir
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/ -run "TestNew|TestRender" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/renderer.go internal/web/renderer_test.go
git commit -m "feat: rewrite renderer with per-page clone pattern and ShellViewModel"
```

---

## Task 2: Create missing template files

**Files:**
- Create: `internal/web/template/pages/auth_login.html` (rename from auth.html)
- Create: `internal/web/template/pages/auth_change_password.html`
- Create: `internal/web/template/pages/auth_reset_password.html`
- Create: `internal/web/template/pages/auth_reset_sent.html`
- Create: `internal/web/template/pages/search.html`
- Create: `internal/web/template/pages/dev_dashboard.html`
- Create: `internal/web/template/partials/message_list.html`
- Create: `internal/web/template/partials/contact_list.html`
- Delete: `internal/web/template/pages/auth.html`

- [ ] **Step 1: Rename auth.html to auth_login.html**

```bash
mv internal/web/template/pages/auth.html internal/web/template/pages/auth_login.html
```

The existing auth.html already has the right structure (login form with CSRF token and error display).

- [ ] **Step 2: Create auth_change_password.html**

```html
{{ define "content" }}
<div class="auth-page">
    <div class="auth-card">
        <h1>Change Password</h1>
        {{ if .Error }}
        <div class="toast toast-error">{{ .Error }}</div>
        {{ end }}
        <form method="POST" action="/auth/change-password">
            <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
            <div class="form-group">
                <label for="current_password">Current Password</label>
                <input type="password" id="current_password" name="current_password" required>
            </div>
            <div class="form-group">
                <label for="new_password">New Password</label>
                <input type="password" id="new_password" name="new_password" required>
            </div>
            <button type="submit" class="btn btn-primary">Change Password</button>
        </form>
    </div>
</div>
{{ end }}
```

- [ ] **Step 3: Create auth_reset_password.html**

```html
{{ define "content" }}
<div class="auth-page">
    <div class="auth-card">
        <h1>Reset Password</h1>
        <p>Enter your email address and we'll send you a reset link.</p>
        {{ if .Error }}
        <div class="toast toast-error">{{ .Error }}</div>
        {{ end }}
        <form method="POST" action="/auth/reset">
            <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" required>
            </div>
            <button type="submit" class="btn btn-primary">Send Reset Link</button>
        </form>
    </div>
</div>
{{ end }}
```

- [ ] **Step 4: Create auth_reset_sent.html**

```html
{{ define "content" }}
<div class="auth-page">
    <div class="auth-card">
        <h1>Check Your Email</h1>
        <p>If an account exists with that email address, a password reset link has been sent.</p>
        <a href="/auth" class="btn btn-secondary">Back to Sign In</a>
    </div>
</div>
{{ end }}
```

- [ ] **Step 5: Create search.html**

```html
{{ define "content" }}
<div class="page-header">
    <h1>Search</h1>
</div>
<form method="GET" action="/search" class="search-form">
    <input type="text" name="q" value="{{ .Query }}" placeholder="Search messages..." autofocus>
    <button type="submit" class="btn btn-primary">Search</button>
</form>
{{ if .Messages }}
<div class="message-list">
    {{ range .Messages }}
    {{ template "message_row" . }}
    {{ end }}
</div>
{{ template "pagination" .Pagination }}
{{ else if .Query }}
<p class="empty-state">No results found for "{{ .Query }}".</p>
{{ end }}
{{ end }}
```

- [ ] **Step 6: Create dev_dashboard.html**

```html
{{ define "content" }}
<div class="page-header">
    <h1>Dev Dashboard</h1>
</div>
<div class="card-grid">
    <div class="card">
        <h2>Outbox</h2>
        <form method="POST" action="/dev/outbox/process">
            <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
            <button type="submit" class="btn btn-primary">Process Pending</button>
        </form>
    </div>
    <div class="card">
        <h2>Sessions</h2>
        <form method="POST" action="/dev/sessions/cleanup">
            <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
            <button type="submit" class="btn btn-primary">Cleanup Expired</button>
        </form>
    </div>
    <div class="card">
        <h2>Cache</h2>
        <form method="POST" action="/dev/cache/invalidate">
            <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
            <button type="submit" class="btn btn-primary">Invalidate Cache</button>
        </form>
    </div>
</div>
{{ end }}
```

- [ ] **Step 7: Create partials/message_list.html**

```html
{{ define "message_list" }}
<div class="message-list">
    {{ range .Messages }}
    {{ template "message_row" . }}
    {{ end }}
</div>
{{ template "pagination" .Pagination }}
{{ end }}
```

- [ ] **Step 8: Create partials/contact_list.html**

```html
{{ define "contact_list" }}
<div class="contact-list">
    {{ range .Contacts }}
    {{ template "contact_row" . }}
    {{ end }}
</div>
{{ template "pagination" .Pagination }}
{{ end }}
```

- [ ] **Step 9: Verify renderer parses all pages**

Run: `go test ./internal/web/ -run TestNewRendererParsesAllPages -v`
Expected: PASS (all pages parse without error)

- [ ] **Step 10: Commit**

```bash
git add internal/web/template/
git commit -m "feat: add missing template files (auth, search, dev dashboard, component partials)"
```

---

## Task 3: Fix base.html — add script and meta tags

**Files:**
- Modify: `internal/web/template/base.html`

- [ ] **Step 1: Add CSRF meta tag and platform.js script**

Edit `internal/web/template/base.html`:

In the `<head>` section, after the stylesheet link, add:
```html
    <meta name="csrf-token" content="{{ .CSRFToken }}">
```

Before `</body>`, add:
```html
    <script src="/static/js/platform.js"></script>
```

The full `<head>` section should look like:
```html
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="csrf-token" content="{{ .CSRFToken }}">
    <title>{{ .Title }} - Outerstellar Platform</title>
    <link rel="stylesheet" href="/static/css/main.css">
    {{ if .CustomCSS }}<style>{{ .CustomCSS }}</style>{{ end }}
</head>
```

And before `</body>`:
```html
    <script src="/static/js/platform.js"></script>
</body>
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/template/base.html
git commit -m "fix: wire CSRF meta tag and platform.js into base layout"
```

---

## Task 4: Create messages handler

**Files:**
- Create: `internal/web/handler/messages.go`
- Modify: `internal/platform/core/core.go` (add MessagesShow to Bundle)
- Modify: `internal/platform/core/contribute.go` (add /messages route + nav)
- Modify: `internal/wire/wire.go` (add MessagesHandler construction + Bundle assignment)

- [ ] **Step 1: Write the messages handler**

```go
// internal/web/handler/messages.go
package handler

import (
	"net/http"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

type MessagesHandler struct {
	messageService *service.MessageService
	renderer       *web.Renderer
}

func NewMessagesHandler(msgSvc *service.MessageService, renderer *web.Renderer) *MessagesHandler {
	return &MessagesHandler{
		messageService: msgSvc,
		renderer:       renderer,
	}
}

func (h *MessagesHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	result, err := h.messageService.ListMessages(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Failed to load messages",
		}, http.StatusInternalServerError)
		return
	}

	messageItems := make([]viewmodel.MessageItem, len(result.Items))
	for i, m := range result.Items {
		messageItems[i] = viewmodel.MessageItem{
			SyncID:       m.SyncID,
			Author:       m.Author,
			Content:      m.Content,
			UpdatedAt:    m.UpdatedAtLabel(),
			UpdatedLabel: m.UpdatedAtLabel(),
			Dirty:        m.Dirty,
			Version:      m.Version,
			HasConflict:  m.HasConflict,
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: result.Metadata.CurrentPage,
		TotalPages:  result.Metadata.TotalPages,
		TotalItems:  result.Metadata.TotalItems,
		HasPrevious: result.Metadata.HasPrevious,
		HasNext:     result.Metadata.HasNext,
		PageSize:    result.Metadata.PageSize,
	}

	if err := h.renderer.RenderPage(w, r, "messages", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
```

- [ ] **Step 2: Add MessagesShow to the Bundle struct**

In `internal/platform/core/core.go`, add this field to the Bundle struct (in the ProtectedUI section):

```go
	MessagesShow         http.HandlerFunc
```

- [ ] **Step 3: Add /messages route and nav to contribute.go**

In `internal/platform/core/contribute.go`, add after the HomeShow protected route:

```go
	ctx.Routes.Protected(http.MethodGet, "/messages", "Messages", http.HandlerFunc(b.MessagesShow))
```

And add to the navigation section:

```go
	ctx.Navigation.Add("Messages", "/messages", "message-square")
```

- [ ] **Step 4: Wire MessagesHandler in wire.go**

In `internal/wire/wire.go`:

1. Add `MessagesHandler *handler.MessagesHandler` to the App struct.

2. After the `searchHandler` construction (around line 175), add:
```go
	messagesHandler := handler.NewMessagesHandler(messageSvc, renderer)
```

3. In the return struct, add:
```go
	MessagesHandler: messagesHandler,
```

4. In `BuildCoreBundle`, add to the ProtectedUI section:
```go
		MessagesShow:         app.MessagesHandler.Show,
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./...`
Expected: Compiles without error.

- [ ] **Step 6: Commit**

```bash
git add internal/web/handler/messages.go internal/platform/core/ internal/wire/wire.go
git commit -m "feat: add messages handler and /messages route"
```

---

## Task 5: Update all handler render calls

This is the mechanical task of changing every `renderer.Render(...)` / `renderer.RenderWithStatus(...)` call to the new `RenderPage` / `RenderWithStatus` / `RenderPartial` signatures.

**Files to modify (every handler with a render call):**
- `internal/web/handler/home.go`
- `internal/web/handler/contacts.go`
- `internal/web/handler/notifications.go`
- `internal/web/handler/settings.go`
- `internal/web/handler/user_admin.go`
- `internal/web/handler/auth.go`
- `internal/web/handler/search.go`
- `internal/web/handler/errors.go`
- `internal/web/handler/components.go`
- `internal/web/handler/dev_dashboard.go`

- [ ] **Step 1: Update home.go**

Change `h.renderer.Render(w, "home.html", page)` to `h.renderer.RenderPage(w, r, "home", page)`.

- [ ] **Step 2: Update contacts.go**

Change all `h.renderer.Render(w, "contacts.html", ...)` to `h.renderer.RenderPage(w, r, "contacts", ...)`.
Change all `h.renderer.RenderWithStatus(w, "error.html", ...)` to `h.renderer.RenderWithStatus(w, r, "error", ...)`.

- [ ] **Step 3: Update notifications.go**

Change `h.renderer.Render(w, "notifications.html", ...)` to `h.renderer.RenderPage(w, r, "notifications", ...)`.
Change error renders similarly.

- [ ] **Step 4: Update settings.go**

Change `h.renderer.Render(w, "settings.html", ...)` to `h.renderer.RenderPage(w, r, "settings", ...)`.

- [ ] **Step 5: Update user_admin.go**

Change `h.renderer.Render(w, "admin_users.html", ...)` to `h.renderer.RenderPage(w, r, "admin_users", ...)`.
Change `h.renderer.Render(w, "admin_audit.html", ...)` to `h.renderer.RenderPage(w, r, "admin_audit", ...)`.
Change error renders similarly.

- [ ] **Step 6: Update auth.go**

Change all `h.renderer.Render(w, "auth_login.html", ...)` to `h.renderer.RenderPage(w, r, "auth_login", ...)`.
Change all `h.renderer.Render(w, "auth_change_password.html", ...)` to `h.renderer.RenderPage(w, r, "auth_change_password", ...)`.
Change all `h.renderer.Render(w, "auth_reset_password.html", ...)` to `h.renderer.RenderPage(w, r, "auth_reset_password", ...)`.
Change all `h.renderer.Render(w, "auth_reset_sent.html", ...)` to `h.renderer.RenderPage(w, r, "auth_reset_sent", ...)`.
Change all `h.renderer.RenderWithStatus(...)` calls to use the new signature with `r *http.Request`.

- [ ] **Step 7: Update search.go**

Change all `h.renderer.Render(w, "search.html", ...)` to `h.renderer.RenderPage(w, r, "search", ...)`.
Change `h.renderer.RenderWithStatus(w, "error.html", ...)` to `h.renderer.RenderWithStatus(w, r, "error", ...)`.

- [ ] **Step 8: Update errors.go**

Change `h.renderer.RenderWithStatus(w, "error.html", ...)` to `h.renderer.RenderWithStatus(w, r, "error", ...)`.

- [ ] **Step 9: Update components.go**

Change `h.renderer.Render(w, "components/message_list.html", ...)` to `h.renderer.RenderPartial(w, "message_list", ...)`.
Change `h.renderer.Render(w, "components/contact_list.html", ...)` to `h.renderer.RenderPartial(w, "contact_list", ...)`.

- [ ] **Step 10: Update dev_dashboard.go**

Change `h.renderer.Render(w, "dev_dashboard.html", nil)` to `h.renderer.RenderPage(w, r, "dev_dashboard", nil)`.

- [ ] **Step 11: Verify compilation**

Run: `go build ./...`
Expected: Compiles without error. If any handler method doesn't have `r *http.Request` available, adjust the method signature.

- [ ] **Step 12: Commit**

```bash
git add internal/web/handler/
git commit -m "refactor: update all handler render calls to new renderer API"
```

---

## Task 6: Update wire.go renderer construction

**Files:**
- Modify: `internal/wire/wire.go`

- [ ] **Step 1: Add version parameter to NewRenderer call**

In `internal/wire/wire.go`, find the `NewRenderer` call (around line 159):

```go
// Before:
renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap())

// After:
renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), cfg.Version)
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles without error.

- [ ] **Step 3: Commit**

```bash
git add internal/wire/wire.go
git commit -m "fix: pass version to NewRenderer"
```

---

## Task 7: Delete dead code

**Files:**
- Delete: `pkg/theme/theme.go`
- Delete: `pkg/theme/shader.go`
- Delete: `pkg/theme/theme_test.go`
- Delete: `internal/model/theme.go`

- [ ] **Step 1: Verify nothing imports these**

Run: `grep -rn "pkg/theme" --include="*.go" .`
Expected: No results.

Run: `grep -rn "ThemeDefinition" --include="*.go" .`
Expected: Only the definition in theme.go (which we're deleting).

- [ ] **Step 2: Delete the files**

```bash
rm -rf pkg/theme/
rm internal/model/theme.go
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Compiles without error.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: delete dead code (pkg/theme/, model.ThemeDefinition)"
```

---

## Task 8: Integration tests — all pages render 200

**Files:**
- Create: `internal/web/handler/render_integration_test.go`

- [ ] **Step 1: Write the integration test**

```go
// internal/web/handler/render_integration_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

// These tests verify that the rendering pipeline produces HTTP 200
// (not 500 from template errors) for every HTML route. They use
// platform.NewTestApp to build the handler through the same assembly
// path as production, but with stub services so no database is needed.
//
// NOTE: The core extension's Bundle is populated with nil HandlerFunc
// values in this test because we're only testing that the RENDERER
// works, not that the handlers return the right data. For a full
// integration test with real handlers, see the end-to-end tests.

func TestRenderPipelineSmoke(t *testing.T) {
	// This is a smoke test: verify the renderer can parse all templates
	// and produce HTML for each page name. The actual route-level
	// integration tests require a wired core extension with real handlers.
	r, err := testRenderer()
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

// testRenderer builds a renderer with the real embedded templates.
func testRenderer() (interface{}, error) {
	// Import the real template FS and build a renderer.
	// This verifies all templates parse correctly.
	return nil, nil // placeholder — implement with actual template FS
}
```

Note: the actual integration test needs to exercise routes through `platform.NewTestApp`. Since the core extension's handlers require real services (database-backed), a true 200-level smoke test requires either (a) stub services or (b) the real test database. For this task, focus on verifying that the renderer parses all templates and that no template has syntax errors:

```go
// internal/web/handler/render_integration_test.go
package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

func TestRendererParsesAllRealTemplates(t *testing.T) {
	templateFS := web.TemplateFS()
	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), "test")
	require.NoError(t, err, "all embedded templates must parse without error")
	assert.NotNil(t, renderer)
}

func TestRendererHasAllExpectedPages(t *testing.T) {
	templateFS := web.TemplateFS()
	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), "test")
	require.NoError(t, err)

	// Every page that handlers render must exist in the renderer.
	expectedPages := []string{
		"home", "contacts", "notifications", "settings",
		"admin_users", "admin_audit", "error",
		"auth_login", "auth_change_password",
		"auth_reset_password", "auth_reset_sent",
		"search", "dev_dashboard", "messages",
	}

	for _, page := range expectedPages {
		// Access the pages map via a test helper or reflection.
		// Since pages is unexported, verify via RenderPage on a stub request.
		assert.True(t, renderer.HasPage(page), "renderer should have page %q", page)
	}
}
```

You'll need to add a `HasPage` method to the Renderer:

```go
// HasPage reports whether the renderer has a parsed template for the given page.
func (r *Renderer) HasPage(name string) bool {
	_, ok := r.pages[name]
	return ok
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/web/... -v -run "TestRenderer"`
Expected: All tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/web/handler/render_integration_test.go internal/web/renderer.go
git commit -m "test: verify renderer parses all templates and has expected pages"
```

---

## Task 9: Final verification

- [ ] **Step 1: Run go build**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No errors.

- [ ] **Step 3: Run all tests (short mode)**

Run: `go test ./... -short -count=1`
Expected: All tests PASS.

- [ ] **Step 4: Run gosec**

Run: `gosec -exclude-dir=internal/persistence/db ./...`
Expected: 0 issues.

- [ ] **Step 5: Commit if any fixups needed**

```bash
git add -A
git commit -m "chore: final verification for rendering pipeline fix"
```
