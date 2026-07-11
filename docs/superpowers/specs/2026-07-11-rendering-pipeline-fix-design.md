# Rendering Pipeline Fix Design (Workstream A)

**Date:** 2026-07-11
**Status:** Approved (pending spec review)
**Workstream:** A of 5 (rendering → security → outbox → observability → stack adoption)

## Problem

The rendering pipeline is fundamentally broken. Zero of ~22 handler render call sites resolve to a defined template. Three compounding failures:

1. **Template name mismatch.** Handlers render names like `"home.html"`, but template files define blocks named `"content"` (not `"home.html"`). Go's `html/template` keys by defined name, not filename, so every `ExecuteTemplate` call fails with "undefined".

2. **No layout wiring.** `base.html` defines `"base"` with a `{{ template "content" .BodyData }}` call, but no handler executes `"base"`. The layout chrome (nav, footer, theme, toasts) is dead code. `ShellViewModel` is never constructed.

3. **`"content"` collision.** All 9 page templates `{{ define "content" }}`. Parsed into one shared set via `ParseFS("**/*.html")`, only the last-parsed survives — the rest are silently overwritten.

Additionally, 7 template files referenced by handlers don't exist, and the `/messages` route has no handler.

## Decisions

| Decision | Choice |
|---|---|
| Template engine | **Fix html/template** (adopt templ in Workstream E) |
| Missing templates | **Create all** — full coverage, every route renders |
| Layout pattern | **Per-page clone** (idiomatic Go template inheritance) |
| Navigation | **Static** in templates for now (extension nav wiring is follow-up) |
| Dead theme service | **Delete `pkg/theme/`** entirely |

## Section 1: The new Renderer

### Structural change

The `Renderer` goes from holding one `*template.Template` to a `map[string]*template.Template`:

```go
type Renderer struct {
    pages    map[string]*template.Template  // "home" → base+partials+home.html
    partials *template.Template              // partials only (for fragment rendering)
    version  string
}
```

### Parsing strategy

`NewRenderer(templateFS fs.FS, funcs template.FuncMap, version string)`:

1. Read `template/base.html` and all `template/partials/*.html` into memory
2. Parse them into a "base" template set (with func map) — this is the layout + shared partials
3. For each `template/pages/*.html` file:
   - Clone the base template set
   - Parse the page file into the clone (its `{{ define "content" }}` overrides cleanly within the isolated clone)
   - Store keyed by page name (filename without `.html` — e.g. `pages/home.html` → `"home"`)
4. Parse all `template/partials/*.html` into a separate `partials` set for fragment rendering

### Rendering methods

```go
// RenderPage renders a page wrapped in the shell layout.
// Builds ShellViewModel from request context automatically.
func (r *Renderer) RenderPage(w http.ResponseWriter, req *http.Request, page string, data interface{}) error

// RenderPartial renders a fragment without shell wrapping (for HTMX responses).
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data interface{}) error

// RenderWithStatus renders a page with a specific HTTP status code.
func (r *Renderer) RenderWithStatus(w http.ResponseWriter, req *http.Request, page string, data interface{}, status int) error
```

### What handlers change to

```go
// Before (broken):
h.renderer.Render(w, "home.html", page)

// After (working):
h.renderer.RenderPage(w, r, "home", page)
```

Handler passes page name (no `.html`) + page viewmodel. Shell construction is automatic.

## Section 2: Shell context extraction

### ShellViewModel construction

`RenderPage` calls `buildShell(req)` which extracts from request context using existing helpers:

| Shell field | Source | Helper |
|---|---|---|
| `User` | Session middleware sets `*model.User` | `web.UserFromRequest(r)` |
| `CSRFToken` | CSRF middleware generates per-request | `web.CSRFTokenFromRequest(r)` |
| `RequestID` | Logging middleware sets request ID | `web.RequestIDFromContext(ctx)` |
| `Theme` | `user.Theme` field (`*string`), default `"light"` | via `UserFromRequest` |
| `IsDark` | `theme == "dark"` | derived |
| `Language` | `user.Language` field | via `UserFromRequest` |
| `Version` | Passed to `NewRenderer` | stored on Renderer |
| `Body` | Page name (for active nav highlighting) | from `RenderPage` argument |
| `BodyData` | Page-specific viewmodel | from `RenderPage` argument |

For unauthenticated requests (login page, etc.), `User` is nil and theme defaults to `"light"`.

### Navigation

Static in templates. `base.html` already has a hardcoded nav bar. The extension model's `NavigationRegistry` is architecturally correct but threading it through to the renderer requires a larger middleware→context→renderer pipeline change. That is a follow-up. For now, nav items are rendered from the existing hardcoded HTML in `base.html`.

## Section 3: Template fixes and new files

### Existing templates: handler call fixes

7 templates exist and work — only the handler render calls need the name changed (drop `.html`):

| Handler file | Current call | Fixed call |
|---|---|---|
| `home.go:52` | `Render(w, "home.html", ...)` | `RenderPage(w, r, "home", ...)` |
| `contacts.go:80` | `Render(w, "contacts.html", ...)` | `RenderPage(w, r, "contacts", ...)` |
| `notifications.go:68` | `Render(w, "notifications.html", ...)` | `RenderPage(w, r, "notifications", ...)` |
| `settings.go:98` | `Render(w, "settings.html", ...)` | `RenderPage(w, r, "settings", ...)` |
| `user_admin.go:74` | `Render(w, "admin_users.html", ...)` | `RenderPage(w, r, "admin_users", ...)` |
| `user_admin.go:193` | `Render(w, "admin_audit.html", ...)` | `RenderPage(w, r, "admin_audit", ...)` |
| All `error.html` calls | `Render(w, "error.html", ...)` / `RenderWithStatus` | `RenderPage` / `RenderWithStatus` with `"error"` |

### New template files (8 files)

1. **`pages/auth_login.html`** — login + register form (renamed from existing `pages/auth.html`; already has correct structure with CSRF token and error display)
2. **`pages/auth_change_password.html`** — change password form (current password, new password, confirm)
3. **`pages/auth_reset_password.html`** — password reset request form (email entry)
4. **`pages/auth_reset_sent.html`** — "if the email exists, a reset link has been sent" confirmation
5. **`pages/search.html`** — search results page (search box + message results list; reuses `MessagesPage` viewmodel but without year-filter UI)
6. **`pages/dev_dashboard.html`** — dev ops dashboard (outbox process button, session cleanup button, cache invalidate button, current stats)
7. **`partials/message_list.html`** — full message list fragment for HTMX (wraps `message_row` partials + pagination)
8. **`partials/contact_list.html`** — full contact list fragment for HTMX (wraps `contact_row` partials + pagination)

### Component handler fix

`components.go` changes from `Render` (which targets pages) to `RenderPartial` (which targets partials):
- `MessageList`: `r.RenderPartial(w, "message_list", page)` instead of `r.Render(w, "components/message_list.html", page)`
- `ContactList`: `r.RenderPartial(w, "contact_list", page)` instead of `r.Render(w, "components/contact_list.html", page)`

### base.html fixes

1. Add `<meta name="csrf-token" content="{{ .CSRFToken }}">` in `<head>` — the JS CSRF fetch wrapper reads this
2. Add `<script src="/static/js/platform.js"></script>` before `</body>` — loads theme toggle, CSRF fetch injection, toast auto-dismiss, confirm dialogs

### Dead code cleanup

- **Delete `pkg/theme/`** (`theme.go`, `shader.go`, `theme_test.go`) — zero imports, CSS naming mismatch, theming works via `data-theme` attribute + user preference
- **Check `internal/model/theme.go`** — if `ThemeDefinition` is unreferenced, delete it too

## Section 4: Messages handler and search fix

### Messages handler (new)

Create `internal/web/handler/messages.go`:

```go
type MessagesHandler struct {
    messageService *service.MessageService
    renderer       *web.Renderer
}

func NewMessagesHandler(msgSvc *service.MessageService, renderer *web.Renderer) *MessagesHandler

func (h *MessagesHandler) Show(w http.ResponseWriter, r *http.Request)
```

`Show` handles `GET /messages`:
- Reads `page` and optional `year` query params
- Calls `messageService.ListMessages(ctx, limit, offset)` for the current page
- Populates `MessagesPage` viewmodel including `Year` and `Years` (derived from message dates)
- Renders `"messages"` page via `RenderPage`

Wiring:
- New `Bundle` field: `MessagesShow http.HandlerFunc`
- New entry in `contribute.go`: `ctx.Routes.Protected(http.MethodGet, "/messages", "Messages", http.HandlerFunc(b.MessagesShow))`
- `wire.go`: `MessagesShow: app.MessagesHandler.Show`
- `buildCoreBundle`: assign the handler

### Search handler fix

`search.go` currently renders `"search.html"` (doesn't exist). Fix:
- Change render call to `RenderPage(w, r, "search", page)` once the `search.html` template exists
- The new `search.html` template reuses `MessagesPage` viewmodel with a search box + results list (no year filter)

### Navigation

Add "Messages" to nav in `contribute.go`:
```go
ctx.Navigation.Add("Messages", "/messages", "message-square")
```

`base.html` already has a `/messages` nav link hardcoded — update its active-class logic to match the `"messages"` page name.

## Section 5: Testing

### Renderer unit tests

- **`TestRendererParsesAllPages`** — `NewRenderer` succeeds, every page file produces a valid cloned template (catches syntax errors and missing files at test time)
- **`TestRenderPageProducesHTML`** — render `"home"` with a stub shell + `HomePage` data, verify output contains `<nav>`, `<title>`, and page-specific content
- **`TestRenderPartialProducesFragment`** — render `"message_list"` partial, verify output does NOT contain `<nav>` or `<title>` (no shell chrome)
- **`TestRenderPageWithUser`** — render with a populated user context, verify `data-theme` and username appear in output

### Handler integration tests

- **`TestAllPagesRender200`** — table-driven test using `platform.NewTestApp` that hits every HTML route (`/`, `/messages`, `/contacts`, `/search`, `/settings`, `/notifications`, `/auth`, `/auth/change-password`, `/auth/reset`, `/admin/users`, `/admin/audit`) and asserts HTTP 200 (not 500 from template errors). This is the end-to-end proof that the rendering pipeline works.
- Component endpoints (`/components/message-list`, `/components/contact-list`) tested separately to verify they return HTML fragments without shell chrome.

### Existing tests preserved

All existing service, model, config, security, platform, and migration tests remain green. No changes to their test code.

## Out of scope for this workstream

- Adopting templ (Workstream E)
- Adopting HTMX for dynamic interactions (Workstream E)
- Wiring extension model navigation registry to the renderer (follow-up)
- Actual text search implementation in the search handler (the handler currently does pagination only; text matching is a follow-up)
- CSP nonce support (Workstream B — security hardening)
- OpenTelemetry tracing (Workstream D)
- Prometheus metrics wiring (Workstream D)
