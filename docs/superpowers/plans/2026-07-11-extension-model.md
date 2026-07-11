# Extension Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retrofit the brief's compile-time extension model (manifest, contribution, route registry with ownership validation, per-extension migrations, platform modes) into the existing gouterstellar-platform codebase.

**Architecture:** A new top-level `platform/` public package defines the extension contract. The `wire.Wire` composition root gains an assembly phase that constructs extensions, collects route contributions, validates ownership/conflicts, and builds the Chi router. Existing handlers become a single `core` extension. A new `reports` extension proves the boundary. The naive migration runner is replaced by a versioned per-extension runner.

**Tech Stack:** Go 1.26, Chi v5, pgx v5, sqlc, testify, testcontainers-go (new), embed.FS

**Spec:** `docs/superpowers/specs/2026-07-11-extension-model-design.md`

---

## File Structure

### New files

| Path | Responsibility |
|---|---|
| `platform/extension.go` | Extension interface, Manifest, PlatformMode, RouteOwnership, MigrationSet types |
| `platform/capabilities.go` | Service capability interfaces (MessageCounter, ContactCounter, etc.) |
| `platform/route_registry.go` | RouteRegistry — registration, collection, ownership binding |
| `platform/route_registry_validate.go` | Validation — all 6 rules, conflict detection, rich diagnostics |
| `platform/route_registry_build.go` | Build validated routes onto Chi router, mode filtering, group middleware |
| `platform/contribution.go` | ContributionContext, NavigationRegistry, AssetRegistry, AdminRegistry |
| `platform/handler.go` | NewHandler — the assembly step (collect → validate → build → return http.Handler) |
| `platform/check.go` | CheckExtension + Diagnostics for contract tests |
| `platform/testapp.go` | NewTestApp + TestOptions for in-memory HTTP tests |
| `platform/migration/runner.go` | Versioned migration runner with per-extension history tables |
| `platform/migration/runner_test.go` | Logic tests (version parsing, ordering, pending filter) |
| `platform/migration/runner_db_test.go` | End-to-end DB tests via Testcontainers |
| `internal/platform/capabilities.go` | Adapters wrapping internal services as platform capability interfaces |
| `internal/platform/core/core.go` | CoreExtension — wraps existing handlers, contributes all routes |
| `internal/platform/core/contribute.go` | Route mapping from existing handlers to registry calls |
| `internal/platform/core/migrations.go` | `go:embed` for core migration SQL files |
| `internal/platform/core/migrations/*.sql` | Moved from `migrations/` (renamed, made idempotent) |
| `extensions/reports/reports.go` | Reports extension — Manifest, Contribute |
| `extensions/reports/handlers.go` | Reports HTTP handlers |
| `extensions/reports/migrations.go` | `go:embed` for reports migration |
| `extensions/reports/migrations/V001__reports_tables.sql` | Reports-specific table |
| `extensions/reports/reports_test.go` | Contract test |
| `extensions/reports/reports_http_test.go` | In-memory HTTP test |

### Modified files

| Path | Change |
|---|---|
| `internal/config/config.go` | Add `PlatformMode` field |
| `internal/service/message_service.go` | Add `CountMessages` passthrough method |
| `internal/wire/wire.go` | Remove plugin manager, add capability adapters, build extension list, add `Extensions` field |
| `cmd/server/main.go` | Replace direct Chi wiring with `platform.NewHandler` |
| `cmd/migrate/main.go` | Use new migration runner |
| `migrations/V1__initial_schema.sql` through `V4__` | Made idempotent (IF NOT EXISTS), then moved |
| `Makefile` | Add `test-integration` target |
| `go.mod` | Add testcontainers-go dependency |

### Deleted files

| Path | Reason |
|---|---|
| `pkg/plugin/plugin.go` | Replaced by extension model |
| `pkg/plugin/manager.go` | Replaced by extension model |
| `pkg/plugin/manager_test.go` | Tests for deleted code |
| `migrations/` (directory) | SQL files moved into core extension's embedded FS |

---

## Task 1: Create the `platform/` package — types and interfaces

**Files:**
- Create: `platform/extension.go`
- Create: `platform/capabilities.go`
- Test: `platform/extension_test.go`

- [ ] **Step 1: Write the failing test**

```go
// platform/extension_test.go
package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name    string
		manifest Manifest
		wantErr bool
	}{
		{
			name: "valid manifest",
			manifest: Manifest{
				ID:    "reports",
				Label: "Reports",
				Mode:  ExtensionHost,
				Ownership: RouteOwnership{
					UI: []string{"/reports"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty ID rejected",
			manifest: Manifest{
				ID:    "",
				Label: "No ID",
				Mode:  FullPlatform,
			},
			wantErr: true,
		},
		{
			name: "invalid mode rejected",
			manifest: Manifest{
				ID:   "x",
				Mode: PlatformMode("bogus"),
			},
			wantErr: true,
		},
		{
			name: "full platform with no ownership rejected",
			manifest: Manifest{
				ID:   "x",
				Mode: FullPlatform,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/ -run TestManifestValidation -v`
Expected: FAIL — package doesn't compile, types undefined.

- [ ] **Step 3: Write the extension types**

```go
// platform/extension.go
package platform

import (
	"fmt"
	"io/fs"
	"strings"
)

// Extension is the contract every compile-time extension satisfies.
type Extension interface {
	Manifest() Manifest
	Contribute(ctx *ContributionContext) error
}

// PlatformMode controls which route groups are mounted and who can own root UI routes.
type PlatformMode string

const (
	FullPlatform  PlatformMode = "full"
	ExtensionHost PlatformMode = "extension-host"
	Headless      PlatformMode = "headless"
)

// Manifest declares an extension's identity, mode, route ownership, and migrations.
type Manifest struct {
	ID         string
	Label      string
	Mode       PlatformMode
	Ownership  RouteOwnership
	Migrations []MigrationSet
}

// RouteOwnership declares the path prefixes an extension is allowed to register under.
type RouteOwnership struct {
	UI     []string
	API    []string
	Admin  []string
	Assets []string
}

// MigrationSet declares an isolated migration history for one extension.
type MigrationSet struct {
	ExtensionID string
	FS          fs.FS
	Directory   string
	Table       string
}

// Validate checks the manifest is well-formed: non-empty ID, valid mode,
// at least one ownership prefix (unless headless).
func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("manifest ID must not be empty")
	}
	switch m.Mode {
	case FullPlatform, ExtensionHost, Headless:
	default:
		return fmt.Errorf("manifest %s: invalid mode %q (want full, extension-host, or headless)", m.ID, m.Mode)
	}
	if m.Mode != Headless {
		if len(m.Ownership.UI) == 0 && len(m.Ownership.API) == 0 &&
			len(m.Ownership.Admin) == 0 && len(m.Ownership.Assets) == 0 {
			return fmt.Errorf("manifest %s: must declare at least one ownership prefix", m.ID)
		}
	}
	return nil
}
```

- [ ] **Step 4: Write the capability interfaces**

```go
// platform/capabilities.go
package platform

import "context"

// MessageCounter is the capability for reading message counts without
// depending on internal service types.
type MessageCounter interface {
	CountMessages(ctx context.Context) (int64, error)
}

// ContactCounter is the capability for reading contact counts.
type ContactCounter interface {
	CountContacts(ctx context.Context) (int64, error)
}

// UserCounter is the capability for reading user counts.
type UserCounter interface {
	CountUsers(ctx context.Context) (int64, error)
}

// ServiceBag carries platform-level capabilities that extensions can request.
// The wire root populates this by adapting internal services.
type ServiceBag struct {
	MessageCounter MessageCounter
	ContactCounter ContactCounter
	UserCounter    UserCounter
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./platform/ -run TestManifestValidation -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add platform/
git commit -m "feat: add platform/ package with extension contract types"
```

---

## Task 2: RouteRegistry — registration, validation, and build

**Files:**
- Create: `platform/route_registry.go`
- Create: `platform/route_registry_validate.go`
- Create: `platform/route_registry_build.go`
- Test: `platform/route_registry_test.go`

- [ ] **Step 1: Write the failing tests — registration**

```go
// platform/route_registry_test.go
package platform

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteRegistryRegistration(t *testing.T) {
	reg := newRouteRegistry("reports")

	reg.Protected(http.MethodGet, "/reports", "Reports home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	reg.API(http.MethodGet, "/api/v1/reports/summary", "Summary", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	routes := reg.All()
	require.Len(t, routes, 2)
	assert.Equal(t, "reports", routes[0].Owner)
	assert.Equal(t, GroupProtectedUI, routes[0].Group)
	assert.Equal(t, GroupAPI, routes[1].Group)
}

func TestRouteRegistryOwnerStamping(t *testing.T) {
	// Each registry instance stamps its owner; routes can't be registered
	// under a different owner.
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/reports", "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, r := range reg.All() {
		assert.Equal(t, "reports", r.Owner, "owner should be stamped from registry construction")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./platform/ -run TestRouteRegistry -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Write the RouteRegistry type and registration**

```go
// platform/route_registry.go
package platform

import "net/http"

// RouteGroup classifies a route for middleware and mode filtering.
type RouteGroup string

const (
	GroupPublicUI    RouteGroup = "public-ui"
	GroupProtectedUI RouteGroup = "protected-ui"
	GroupAPI         RouteGroup = "api"
	GroupAdmin       RouteGroup = "admin"
	GroupAssets      RouteGroup = "assets"
)

// RouteRegistration is a single route contributed by an extension.
type RouteRegistration struct {
	Owner       string
	Method      string
	Pattern     string
	Group       RouteGroup
	Description string
	Handler     http.Handler
}

// RouteRegistry collects route registrations during extension contribution.
// No Chi router is touched until Build is called.
type RouteRegistry struct {
	owner  string
	routes []RouteRegistration
}

// newRouteRegistry creates a registry that stamps every registration
// with the given owner (extension ID).
func newRouteRegistry(owner string) *RouteRegistry {
	return &RouteRegistry{owner: owner}
}

// Public registers a public UI route (no auth required, e.g. login page).
func (r *RouteRegistry) Public(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupPublicUI)
}

// Protected registers a protected UI route (auth required, HTML response).
func (r *RouteRegistry) Protected(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupProtectedUI)
}

// API registers a JSON API route (bearer auth applied by builder).
func (r *RouteRegistry) API(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupAPI)
}

// Admin registers an admin-only UI route.
func (r *RouteRegistry) Admin(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupAdmin)
}

// Assets registers a static asset handler.
func (r *RouteRegistry) Assets(pattern string, h http.Handler) {
	r.add(http.MethodGet, pattern, "static assets", h, GroupAssets)
}

func (r *RouteRegistry) add(method, pattern, desc string, h http.Handler, group RouteGroup) {
	r.routes = append(r.routes, RouteRegistration{
		Owner:       r.owner,
		Method:      method,
		Pattern:     pattern,
		Group:       group,
		Description: desc,
		Handler:     h,
	})
}

// All returns all collected registrations (for inspection/logging).
func (r *RouteRegistry) All() []RouteRegistration {
	return r.routes
}

// ByOwner returns registrations contributed by the given extension ID.
func (r *RouteRegistry) ByOwner(id string) []RouteRegistration {
	var result []RouteRegistration
	for _, r := range r.routes {
		if r.Owner == id {
			result = append(result, r)
		}
	}
	return result
}

// Find returns the registration for a method+pattern, if it exists.
func (r *RouteRegistry) Find(method, pattern string) (RouteRegistration, bool) {
	for _, reg := range r.routes {
		if reg.Method == method && reg.Pattern == pattern {
			return reg, true
		}
	}
	return RouteRegistration{}, false
}
```

- [ ] **Step 4: Run registration tests to verify they pass**

Run: `go test ./platform/ -run TestRouteRegistryRegistration TestRouteRegistryOwnerStamping -v`
Expected: PASS

- [ ] **Step 5: Write the failing tests — validation**

```go
// (append to platform/route_registry_test.go)

func TestValidateAbsolutePaths(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "relative/path", "", stubHandler()) // rule 1

	errs := validateRoutes(reg.All(), FullPlatform, nil)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "must be absolute")
}

func TestValidateOwnership(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/settings", "", stubHandler()) // outside ownership

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}},
	}
	errs := validateRoutes(reg.All(), FullPlatform, ownership)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "outside ownership")
}

func TestValidateConflictDetection(t *testing.T) {
	reg1 := newRouteRegistry("platform-core")
	reg1.Protected(http.MethodGet, "/reports", "", stubHandler())

	reg2 := newRouteRegistry("reports")
	reg2.Protected(http.MethodGet, "/reports", "", stubHandler())

	all := append(reg1.All(), reg2.All...)
	ownership := map[string]RouteOwnership{
		"platform-core": {UI: []string{"/"}},
		"reports":       {UI: []string{"/reports"}},
	}
	errs := validateRoutes(all, FullPlatform, ownership)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "route conflict: GET /reports")
	assert.Contains(t, errs[0].Error(), "platform-core")
	assert.Contains(t, errs[0].Error(), "reports")
}

func TestValidateHeadlessRejectsHTML(t *testing.T) {
	reg := newRouteRegistry("platform-core")
	reg.Protected(http.MethodGet, "/", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"platform-core": {UI: []string{"/"}},
	}
	errs := validateRoutes(reg.All(), Headless, ownership)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "headless mode rejects HTML route")
}

func TestValidateCollectsAllErrors(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "relative", "", stubHandler())  // rule 1
	reg.Protected(http.MethodGet, "/outside", "", stubHandler())   // rule 2

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}},
	}
	errs := validateRoutes(reg.All(), FullPlatform, ownership)
	assert.Len(t, errs, 2, "should collect all errors, not abort on first")
}

func TestValidateAcceptsOwnedRoute(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/reports/home", "", stubHandler())
	reg.API(http.MethodGet, "/api/v1/reports/summary", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}, API: []string{"/api/v1", "/api/v1/reports"}},
	}
	errs := validateRoutes(reg.All(), FullPlatform, ownership)
	assert.Empty(t, errs)
}

func stubHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}
```

- [ ] **Step 6: Run validation tests to verify they fail**

Run: `go test ./platform/ -run TestValidate -v`
Expected: FAIL — `validateRoutes` undefined.

- [ ] **Step 7: Write the validation logic**

```go
// platform/route_registry_validate.go
package platform

import (
	"fmt"
	"strings"
)

// routeError is a validation error with enough context to debug.
type routeError struct {
	Owner   string
	Method  string
	Pattern string
	Detail  string
}

func (e routeError) Error() string {
	return fmt.Sprintf("%s %s (%s): %s", e.Method, e.Pattern, e.Owner, e.Detail)
}

// validateRoutes checks all 6 validation rules and returns ALL errors found,
// not just the first. Returns nil if everything is valid.
//
// Rules:
//  1. Every path is absolute
//  2. Route is inside the owner's declared prefixes
//  3. No two owners claim the same method+path
//  4. Headless mode does not mount HTML routes
//  5. (platform page conflicts are checked at the handler level)
//  6. Asset paths remain inside declared asset ownership
func validateRoutes(routes []RouteRegistration, mode PlatformMode, ownership map[string]RouteOwnership) []error {
	var errs []error

	// Index for duplicate detection (rule 3)
	type routeKey struct {
		method  string
		pattern string
	}
	seen := make(map[routeKey][]string) // key → list of owners

	for _, r := range routes {
		// Rule 1: absolute path
		if !strings.HasPrefix(r.Pattern, "/") {
			errs = append(errs, routeError{
				Owner: r.Owner, Method: r.Method, Pattern: r.Pattern,
				Detail: fmt.Sprintf("route path must be absolute: %q", r.Pattern),
			})
			continue // skip further checks for this route
		}

		// Rule 4: headless rejects HTML groups
		if mode == Headless && (r.Group == GroupPublicUI || r.Group == GroupProtectedUI || r.Group == GroupAdmin) {
			errs = append(errs, routeError{
				Owner: r.Owner, Method: r.Method, Pattern: r.Pattern,
				Detail: "headless mode rejects HTML route",
			})
			continue
		}

		// Rule 2 & 6: ownership check
		ownerPrefixes := getPrefixes(ownership, r.Owner, r.Group)
		if !routeInsideOwnership(r.Pattern, ownerPrefixes) {
			errs = append(errs, routeError{
				Owner: r.Owner, Method: r.Method, Pattern: r.Pattern,
				Detail: fmt.Sprintf("outside ownership (allowed: %s)", strings.Join(ownerPrefixes, ", ")),
			})
		}

		// Rule 3: duplicate detection
		key := routeKey{r.Method, r.Pattern}
		seen[key] = append(seen[key], r.Owner)
	}

	// Report all conflicts (rule 3)
	for key, owners := range seen {
		if len(owners) > 1 {
			errs = append(errs, fmt.Errorf(
				"route conflict: %s %s is owned by both %s",
				key.method, key.pattern, strings.Join(owners, " and "),
			))
		}
	}

	return errs
}

func getPrefixes(ownership map[string]RouteOwnership, owner string, group RouteGroup) []string {
	o, ok := ownership[owner]
	if !ok {
		return nil
	}
	switch group {
	case GroupPublicUI, GroupProtectedUI:
		return o.UI
	case GroupAPI:
		return o.API
	case GroupAdmin:
		return o.Admin
	case GroupAssets:
		return o.Assets
	default:
		return nil
	}
}

// routeInsideOwnership checks whether the pattern starts with (or equals)
// one of the declared ownership prefixes.
func routeInsideOwnership(pattern string, prefixes []string) bool {
	for _, p := range prefixes {
		if pattern == p {
			return true
		}
		// Route is inside the prefix if it shares the prefix boundary:
		// "/reports" owns "/reports", "/reports/home", "/reports/x"
		// but NOT "/reports-malicious"
		if strings.HasPrefix(pattern, p+"/") {
			return true
		}
		// A prefix of "/" owns everything
		if p == "/" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 8: Run validation tests to verify they pass**

Run: `go test ./platform/ -run TestValidate -v`
Expected: PASS

- [ ] **Step 9: Write the failing test — build to Chi**

```go
// (append to platform/route_registry_test.go)

func TestBuildMountsRoutes(t *testing.T) {
	reg := newRouteRegistry("reports")
	called := false
	reg.Protected(http.MethodGet, "/reports", "home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}},
	}

	r := chi.NewRouter()
	errs := buildRoutes(r, reg.All(), FullPlatform, ownership)
	require.Empty(t, errs)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, called, "handler should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBuildHeadlessDropsHTMLRoutes(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/reports", "", stubHandler()) // HTML, dropped
	reg.API(http.MethodGet, "/api/v1/reports/summary", "", stubHandler()) // API, kept

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}, API: []string{"/api/v1"}},
	}

	r := chi.NewRouter()
	mounted := buildRoutes(r, reg.All(), Headless, ownership)

	// In headless mode, HTML routes should NOT be mounted
	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "HTML route should not be mounted in headless mode")

	// API route should be mounted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code, "API route should be mounted in headless mode")

	_ = mounted
}
```

- [ ] **Step 10: Run build tests to verify they fail**

Run: `go test ./platform/ -run TestBuild -v`
Expected: FAIL — `buildRoutes` undefined.

- [ ] **Step 11: Write the build logic**

```go
// platform/route_registry_build.go
package platform

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// buildRoutes mounts validated route registrations onto a Chi router.
// In Headless mode, HTML groups are skipped.
// Returns the list of routes that were actually mounted.
func buildRoutes(r chi.Router, routes []RouteRegistration, mode PlatformMode, ownership map[string]RouteOwnership) []RouteRegistration {
	var mounted []RouteRegistration

	for _, reg := range routes {
		// Headless mode skips HTML + admin groups
		if mode == Headless && isHTMLGroup(reg.Group) {
			continue
		}

		switch reg.Method {
		case http.MethodGet:
			r.Get(reg.Pattern, reg.Handler.ServeHTTP)
		case http.MethodPost:
			r.Post(reg.Pattern, reg.Handler.ServeHTTP)
		case http.MethodPut:
			r.Put(reg.Pattern, reg.Handler.ServeHTTP)
		case http.MethodDelete:
			r.Delete(reg.Pattern, reg.Handler.ServeHTTP)
		case http.MethodPatch:
			r.Patch(reg.Pattern, reg.Handler.ServeHTTP)
		default:
			r.Handle(reg.Pattern, reg.Handler)
		}

		mounted = append(mounted, reg)
	}

	return mounted
}

func isHTMLGroup(g RouteGroup) bool {
	return g == GroupPublicUI || g == GroupProtectedUI || g == GroupAdmin
}
```

- [ ] **Step 12: Run build tests to verify they pass**

Run: `go test ./platform/ -run TestBuild -v`
Expected: PASS

- [ ] **Step 13: Run all platform tests to verify everything passes**

Run: `go test ./platform/ -v`
Expected: All tests PASS.

- [ ] **Step 14: Commit**

```bash
git add platform/route_registry*.go platform/route_registry_test.go
git commit -m "feat: add route registry with validation and Chi build"
```

---

## Task 3: ContributionContext and registries

**Files:**
- Create: `platform/contribution.go`
- Test: `platform/contribution_test.go`

- [ ] **Step 1: Write the failing test**

```go
// platform/contribution_test.go
package platform

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContributionContextNavigation(t *testing.T) {
	nav := NewNavigationRegistry()
	nav.Add("Reports", "/reports", "bar-chart")
	nav.Add("Settings", "/settings", "gear")

	items := nav.Items()
	require.Len(t, items, 2)
	assert.Equal(t, "Reports", items[0].Label)
	assert.Equal(t, "/reports", items[0].URL)
	assert.Equal(t, "bar-chart", items[0].Icon)
}

func TestNewContributionForOwner(t *testing.T) {
	ctx := NewContributionContext("reports")

	ctx.Routes.Protected(http.MethodGet, "/reports", "home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ctx.Navigation.Add("Reports", "/reports", "bar-chart")

	// Verify owner stamping
	routes := ctx.Routes.All()
	require.Len(t, routes, 1)
	assert.Equal(t, "reports", routes[0].Owner)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/ -run TestContribution -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Write the contribution types**

```go
// platform/contribution.go
package platform

// NavigationItem is a single nav entry contributed by an extension.
type NavigationItem struct {
	Label string
	URL   string
	Icon  string
}

// NavigationRegistry collects navigation items during contribution.
type NavigationRegistry struct {
	items []NavigationItem
}

func NewNavigationRegistry() *NavigationRegistry {
	return &NavigationRegistry{}
}

func (n *NavigationRegistry) Add(label, url, icon string) {
	n.items = append(n.items, NavigationItem{Label: label, URL: url, Icon: icon})
}

func (n *NavigationRegistry) Items() []NavigationItem {
	return n.items
}

// AssetRegistry collects static asset declarations.
type AssetRegistry struct {
	entries []AssetEntry
}

type AssetEntry struct {
	Pattern string
	Path    string
}

func NewAssetRegistry() *AssetRegistry {
	return &AssetRegistry{}
}

func (a *AssetRegistry) Add(pattern, path string) {
	a.entries = append(a.entries, AssetEntry{Pattern: pattern, Path: path})
}

func (a *AssetRegistry) Entries() []AssetEntry {
	return a.entries
}

// AdminRegistry collects admin page contributions.
type AdminRegistry struct {
	pages []AdminPage
}

type AdminPage struct {
	Label string
	URL   string
}

func NewAdminRegistry() *AdminRegistry {
	return &AdminRegistry{}
}

func (a *AdminRegistry) Add(label, url string) {
	a.pages = append(a.pages, AdminPage{Label: label, URL: url})
}

func (a *AdminRegistry) Pages() []AdminPage {
	return a.pages
}

// ContributionContext is the capability surface passed to Extension.Contribute.
// Each instance is constructed per-extension and stamps the owner ID onto
// every route registration.
type ContributionContext struct {
	Routes     *RouteRegistry
	Navigation *NavigationRegistry
	Assets     *AssetRegistry
	Admin      *AdminRegistry
}

// NewContributionContext builds a context for a specific extension owner.
func NewContributionContext(owner string) *ContributionContext {
	return &ContributionContext{
		Routes:     newRouteRegistry(owner),
		Navigation: NewNavigationRegistry(),
		Assets:     NewAssetRegistry(),
		Admin:      NewAdminRegistry(),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./platform/ -run TestContribution -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add platform/contribution.go platform/contribution_test.go
git commit -m "feat: add ContributionContext with navigation and asset registries"
```

---

## Task 4: `platform.NewHandler` — the assembly step

**Files:**
- Create: `platform/handler.go`
- Test: `platform/handler_test.go`

- [ ] **Step 1: Write the failing test**

```go
// platform/handler_test.go
package platform

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubExtension is a minimal extension for testing.
type stubExtension struct {
	manifest Manifest
	contrib  func(*ContributionContext) error
}

func (e *stubExtension) Manifest() Manifest { return e.manifest }
func (e *stubExtension) Contribute(ctx *ContributionContext) error {
	if e.contrib != nil {
		return e.contrib(ctx)
	}
	return nil
}

func TestNewHandlerAssemblesRoutes(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "test-ext", Label: "Test", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/test"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/test", "test page",
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("hello from test-ext"))
				}))
			return nil
		},
	}

	handler, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "hello from test-ext")
}

func TestNewHandlerRejectsInvalidOwnership(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "bad-ext", Label: "Bad", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/allowed"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/not-allowed", "", stubHandler())
			return nil
		},
	}

	_, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside ownership")
}

func TestNewHandlerRejectsConflict(t *testing.T) {
	ext1 := &stubExtension{
		manifest: Manifest{
			ID: "ext-a", Label: "A", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/shared"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/shared", "", stubHandler())
			return nil
		},
	}
	ext2 := &stubExtension{
		manifest: Manifest{
			ID: "ext-b", Label: "B", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/shared"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/shared", "", stubHandler())
			return nil
		},
	}

	_, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext1, ext2},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route conflict: GET /shared")
	assert.Contains(t, err.Error(), "ext-a")
	assert.Contains(t, err.Error(), "ext-b")
}

func TestNewHandlerContributeError(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "err-ext", Label: "Err", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/x"}},
		},
		contrib: func(ctx *ContributionContext) error {
			return errors.New("extension init failed")
		},
	}

	_, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extension err-ext")
}

func TestNewHandlerAppliesMiddlewareChain(t *testing.T) {
	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	ext := &stubExtension{
		manifest: Manifest{
			ID: "mw-ext", Label: "MW", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/mw"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/mw", "", stubHandler())
			return nil
		},
	}

	handler, err := NewHandler(Options{
		Mode:             FullPlatform,
		Extensions:       []Extension{ext},
		MiddlewareChain:  []func(http.Handler) http.Handler{mw},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/mw", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "middleware should have been called")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./platform/ -run TestNewHandler -v`
Expected: FAIL — `NewHandler`, `Options` undefined.

- [ ] **Step 3: Write the assembly handler**

```go
// platform/handler.go
package platform

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
)

// Options configures the platform handler assembly.
type Options struct {
	Mode            PlatformMode
	Extensions      []Extension
	Services        ServiceBag
	MiddlewareChain []func(http.Handler) http.Handler
}

// NewHandler assembles the complete web application as an http.Handler.
// It: validates manifests → runs contribution → validates routes → builds router.
// Returns an error if any validation fails; the error includes ALL failures.
func NewHandler(opts Options) (http.Handler, error) {
	// 1. Validate all manifests first.
	ownershipMap := make(map[string]RouteOwnership)
	var allRoutes []RouteRegistration
	var allNav []NavigationItem

	for _, ext := range opts.Extensions {
		m := ext.Manifest()
		if err := m.Validate(); err != nil {
			return nil, fmt.Errorf("extension %s: %w", m.ID, err)
		}
		ownershipMap[m.ID] = m.Ownership
	}

	// 2. Run contribution for each extension.
	for _, ext := range opts.Extensions {
		ctx := NewContributionContext(ext.Manifest().ID)
		if err := ext.Contribute(ctx); err != nil {
			return nil, fmt.Errorf("extension %s contribute: %w", ext.Manifest().ID, err)
		}
		allRoutes = append(allRoutes, ctx.Routes.All()...)
		allNav = append(allNav, ctx.Navigation.Items()...)
	}

	// 3. Validate all routes against ownership + conflicts.
	errs := validateRoutes(allRoutes, opts.Mode, ownershipMap)
	if len(errs) > 0 {
		return nil, formatValidationErrors(errs)
	}

	// 4. Build the Chi router.
	r := chi.NewRouter()

	for _, mw := range opts.MiddlewareChain {
		r.Use(mw)
	}

	mounted := buildRoutes(r, allRoutes, opts.Mode, ownershipMap)

	// 5. Log the route table (observability).
	logRouteTable(mounted, allNav)

	return r, nil
}

// formatValidationErrors turns a slice of errors into a single actionable error.
func formatValidationErrors(errs []error) error {
	msg := fmt.Sprintf("%d route validation error(s):\n", len(errs))
	for _, e := range errs {
		msg += fmt.Sprintf("  - %s\n", e.Error())
	}
	return fmt.Errorf("%s", msg)
}

// logRouteTable emits the validated routing table at startup.
func logRouteTable(routes []RouteRegistration, nav []NavigationItem) {
	// Group by owner
	byOwner := make(map[string][]RouteRegistration)
	for _, r := range routes {
		byOwner[r.Owner] = append(byOwner[r.Owner], r)
	}

	owners := make([]string, 0, len(byOwner))
	for k := range byOwner {
		owners = append(owners, k)
	}
	sort.Strings(owners)

	slog.Info("route registry validated",
		"routes", len(routes),
		"extensions", len(owners),
	)

	for _, owner := range owners {
		for _, r := range byOwner[owner] {
			slog.Debug("route mounted",
				"owner", owner,
				"method", r.Method,
				"pattern", r.Pattern,
				"group", r.Group,
				"description", r.Description,
			)
		}
	}

	if len(nav) > 0 {
		slog.Info("navigation items contributed", "count", len(nav))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./platform/ -run TestNewHandler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add platform/handler.go platform/handler_test.go
git commit -m "feat: add platform.NewHandler assembly step"
```

---

## Task 5: `CheckExtension` and `NewTestApp` — test infrastructure

**Files:**
- Create: `platform/check.go`
- Create: `platform/testapp.go`
- Test: `platform/check_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// platform/check_test.go
package platform

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckExtensionValid(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "reports", Label: "Reports", Mode: ExtensionHost,
			Ownership: RouteOwnership{UI: []string{"/reports"}, API: []string{"/api/v1/reports"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/reports", "home", stubHandler())
			ctx.Routes.API(http.MethodGet, "/api/v1/reports/summary", "summary", stubHandler())
			ctx.Navigation.Add("Reports", "/reports", "bar-chart")
			return nil
		},
	}

	diag, err := CheckExtension(ext, TestHostContext())
	require.NoError(t, err)

	assert.Equal(t, []string{"GET /reports", "GET /api/v1/reports/summary"}, diag.RoutePatterns())
	assert.Contains(t, diag.NavigationLabels(), "Reports")
	assert.NoError(t, diag.OwnershipViolations())
}

func TestCheckExtensionOwnershipViolation(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "reports", Label: "Reports", Mode: ExtensionHost,
			Ownership: RouteOwnership{UI: []string{"/reports"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/settings", "", stubHandler()) // outside!
			return nil
		},
	}

	_, err := CheckExtension(ext, TestHostContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside ownership")
}

func TestCheckExtensionInternalDuplicate(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "reports", Label: "Reports", Mode: ExtensionHost,
			Ownership: RouteOwnership{UI: []string{"/reports"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/reports", "", stubHandler())
			ctx.Routes.Protected(http.MethodGet, "/reports", "", stubHandler()) // duplicate!
			return nil
		},
	}

	_, err := CheckExtension(ext, TestHostContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route conflict")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./platform/ -run TestCheckExtension -v`
Expected: FAIL — `CheckExtension`, `Diagnostics`, `TestHostContext` undefined.

- [ ] **Step 3: Write `CheckExtension` and `Diagnostics`**

```go
// platform/check.go
package platform

import (
	"fmt"
	"strings"
)

// Diagnostics is a read-only summary of what an extension contributed.
// Used by contract tests (Tier 1).
type Diagnostics struct {
	routes         []RouteRegistration
	navLabels      []string
	ownershipErrs  []error
}

// RoutePatterns returns "METHOD PATTERN" for each contributed route.
func (d Diagnostics) RoutePatterns() []string {
	patterns := make([]string, len(d.routes))
	for i, r := range d.routes {
		patterns[i] = fmt.Sprintf("%s %s", r.Method, r.Pattern)
	}
	return patterns
}

// NavigationLabels returns the labels of contributed navigation items.
func (d Diagnostics) NavigationLabels() []string {
	return d.navLabels
}

// OwnershipViolations returns nil if all routes are inside the extension's
// declared ownership, or an error summarizing violations.
func (d Diagnostics) OwnershipViolations() error {
	if len(d.ownershipErrs) == 0 {
		return nil
	}
	msgs := make([]string, len(d.ownershipErrs))
	for i, e := range d.ownershipErrs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("ownership violations: %s", strings.Join(msgs, "; "))
}

// CheckExtension validates a single extension's manifest and contributions
// without starting the platform. It constructs a throwaway ContributionContext,
// runs Contribute, and checks: manifest well-formedness, ownership compliance,
// and internal duplicate routes.
func CheckExtension(ext Extension, ctx *ContributionContext) (*Diagnostics, error) {
	m := ext.Manifest()
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", m.ID, err)
	}

	if err := ext.Contribute(ctx); err != nil {
		return nil, fmt.Errorf("contribute %s: %w", m.ID, err)
	}

	routes := ctx.Routes.All()
	ownership := map[string]RouteOwnership{m.ID: m.Ownership}
	ownershipErrs := validateRoutes(routes, FullPlatform, ownership)

	diag := &Diagnostics{
		routes:        routes,
		navLabels:     navLabelSlice(ctx.Navigation.Items()),
		ownershipErrs: ownershipErrs,
	}

	// Ownership violations are returned as errors (fail the check)
	if len(ownershipErrs) > 0 {
		return diag, diag.OwnershipViolations()
	}

	return diag, nil
}

// TestHostContext creates a ContributionContext suitable for contract tests.
func TestHostContext() *ContributionContext {
	return NewContributionContext("test-host")
}

func navLabelSlice(items []NavigationItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}
```

- [ ] **Step 4: Write `NewTestApp`**

```go
// platform/testapp.go
package platform

import (
	"net/http"
)

// TestOptions configures a test application built through the same assembly
// path as production, but with lightweight stubs instead of real database services.
type TestOptions struct {
	Mode            PlatformMode
	Extensions      []Extension
	MiddlewareChain []func(http.Handler) http.Handler
}

// TestApp is the result of NewTestApp. Close releases any resources.
type TestApp struct {
	Handler http.Handler
}

// NewTestApp builds the handler through the same NewHandler assembly path
// as production, using the same validation and build steps.
func NewTestApp(opts TestOptions) (*TestApp, error) {
	handler, err := NewHandler(Options{
		Mode:            opts.Mode,
		Extensions:      opts.Extensions,
		MiddlewareChain: opts.MiddlewareChain,
	})
	if err != nil {
		return nil, err
	}

	return &TestApp{Handler: handler}, nil
}

// Close releases test app resources (currently a no-op; extensions own their cleanup).
func (a *TestApp) Close() {}
```

- [ ] **Step 5: Run all platform tests to verify they pass**

Run: `go test ./platform/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add platform/check.go platform/testapp.go platform/check_test.go
git commit -m "feat: add CheckExtension contract tests and NewTestApp helper"
```

---

## Task 6: Capability adapters (internal services → platform interfaces)

**Files:**
- Create: `internal/platform/capabilities.go`
- Modify: `internal/service/message_service.go` (add `CountMessages` passthrough)

- [ ] **Step 1: Add `CountMessages` to MessageService**

Add this method to `internal/service/message_service.go` (append to the file, after the existing methods). First check the `MessageRepository` interface has `CountMessages`:

```go
// internal/service/message_service.go — add this method:

// CountMessages returns the total number of non-deleted messages.
func (s *MessageService) CountMessages(ctx context.Context) (int64, error) {
	return s.repo.CountMessages(ctx)
}
```

Note: the `repo` field on `MessageService` is `persistence.MessageRepository` which already has `CountMessages(ctx) (int64, error)`.

- [ ] **Step 2: Write the capability adapters**

```go
// internal/platform/capabilities.go
package platform

import (
	"context"

	"github.com/rygel/gouterstellar-platform/internal/service"
)

// MessageCounterAdapter wraps *service.MessageService as a platform.MessageCounter.
type MessageCounterAdapter struct {
	svc *service.MessageService
}

func NewMessageCounterAdapter(svc *service.MessageService) *MessageCounterAdapter {
	return &MessageCounterAdapter{svc: svc}
}

func (a *MessageCounterAdapter) CountMessages(ctx context.Context) (int64, error) {
	return a.svc.CountMessages(ctx)
}

// ContactCounterAdapter wraps *service.ContactService as a platform.ContactCounter.
type ContactCounterAdapter struct {
	svc *service.ContactService
}

func NewContactCounterAdapter(svc *service.ContactService) *ContactCounterAdapter {
	return &ContactCounterAdapter{svc: svc}
}

func (a *ContactCounterAdapter) CountContacts(ctx context.Context) (int64, error) {
	return a.svc.CountContacts(ctx)
}

// UserCounterAdapter wraps *service.SecurityService as a platform.UserCounter.
type UserCounterAdapter struct {
	svc *service.SecurityService
}

func NewUserCounterAdapter(svc *service.SecurityService) *UserCounterAdapter {
	return &UserCounterAdapter{svc: svc}
}

func (a *UserCounterAdapter) CountUsers(ctx context.Context) (int64, error) {
	return a.svc.CountUsers(ctx)
}

// BuildServiceBag creates a platform.ServiceBag from the internal services.
func BuildServiceBag(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	secSvc *service.SecurityService,
) platform.ServiceBag {
	return platform.ServiceBag{
		MessageCounter: NewMessageCounterAdapter(msgSvc),
		ContactCounter: NewContactCounterAdapter(contactSvc),
		UserCounter:    NewUserCounterAdapter(secSvc),
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/platform/`
Expected: Compiles without error.

Note: This package is `internal/platform` (Go package name `platform` within `internal/`). It imports the top-level `platform` package. This creates a naming situation — the import path differs. Use a qualified import if needed:

```go
import (
	platform "github.com/rygel/gouterstellar-platform/platform"
)
```

If the package name conflicts, rename the import to `extapi` or similar. The adapter code above assumes a clean import; adjust the qualifier if Go complains.

- [ ] **Step 4: Commit**

```bash
git add internal/platform/capabilities.go internal/service/message_service.go
git commit -m "feat: add capability adapters bridging internal services to platform interfaces"
```

---

## Task 7: Migration runner — logic and DB execution

**Files:**
- Create: `platform/migration/runner.go`
- Test: `platform/migration/runner_test.go`
- Test: `platform/migration/runner_db_test.go`

- [ ] **Step 1: Write the failing logic tests**

```go
// platform/migration/runner_test.go
package migration

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		filename string
		wantVer  int64
		wantErr  bool
	}{
		{"V001__initial_schema.sql", 1, false},
		{"V002__user_profiles.sql", 2, false},
		{"V010__add_indexes.sql", 10, false},
		{"V100__big_migration.sql", 100, false},
		{"V1__old_format.sql", 1, false},
		{"not_a_migration.sql", 0, true},
		{"V.sql", 0, true},
		{"README.md", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ver, err := parseVersion(tt.filename)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantVer, ver)
			}
		})
	}
}

func TestSortSetsOrder(t *testing.T) {
	sets := []setEntry{
		{extensionID: "zebra"},
		{extensionID: "reports"},
		{extensionID: "platform-core"},
	}
	sort.Sort(byExtensionID(sets))

	assert.Equal(t, "platform-core", sets[0].extensionID)
	assert.Equal(t, "reports", sets[1].extensionID)
	assert.Equal(t, "zebra", sets[2].extensionID)
}

func TestPendingMigrationsFiltersApplied(t *testing.T) {
	files := []migrationFile{
		{Version: 1, Filename: "V001__a.sql"},
		{Version: 2, Filename: "V002__b.sql"},
		{Version: 3, Filename: "V003__c.sql"},
	}
	applied := map[int64]bool{1: true, 2: true}

	pending := pendingFiles(files, applied)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(3), pending[0].Version)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./platform/migration/ -v`
Expected: FAIL — package doesn't compile.

- [ ] **Step 3: Write the runner**

```go
// platform/migration/runner.go
package migration

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

var versionPattern = regexp.MustCompile(`^V(\d+)__.*\.sql$`)

// migrationFile is a parsed SQL migration file.
type migrationFile struct {
	Version  int64
	Filename string
	Content  string
}

// setEntry is an internal representation of a MigrationSet with its parsed files.
type setEntry struct {
	extensionID string
	table       string
	fs          fs.FS
	dir         string
}

// byExtensionID sorts setEntries by extension ID (platform-core always first).
type byExtensionID []setEntry

func (s byExtensionID) Len() int { return len(s) }
func (s byExtensionID) Less(i, j int) bool {
	// platform-core always sorts first
	if s[i].extensionID == "platform-core" {
		return true
	}
	if s[j].extensionID == "platform-core" {
		return false
	}
	return s[i].extensionID < s[j].extensionID
}
func (s byExtensionID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Runner applies migrations for all extension migration sets in deterministic order.
type Runner struct {
	pool *pgxpool.Pool
	sets []setEntry
}

// NewRunner creates a migration runner from platform MigrationSets.
func NewRunner(pool *pgxpool.Pool, sets []platform.MigrationSet) *Runner {
	entries := make([]setEntry, len(sets))
	for i, s := range sets {
		entries[i] = setEntry{
			extensionID: s.ExtensionID,
			table:       s.Table,
			fs:          s.FS,
			dir:         s.Directory,
		}
	}
	sort.Sort(byExtensionID(entries))
	return &Runner{pool: pool, sets: entries}
}

// Run applies all pending migrations in deterministic order.
func (r *Runner) Run(ctx context.Context) error {
	for _, set := range r.sets {
		if err := r.runSet(ctx, set); err != nil {
			return fmt.Errorf("migration set %s: %w", set.extensionID, err)
		}
	}
	return nil
}

func (r *Runner) runSet(ctx context.Context, set setEntry) error {
	if err := r.ensureHistoryTable(ctx, set); err != nil {
		return err
	}

	applied, err := r.appliedVersions(ctx, set)
	if err != nil {
		return err
	}

	files, err := readMigrationFiles(set)
	if err != nil {
		return err
	}

	pending := pendingFiles(files, applied)

	for _, m := range pending {
		if err := r.applyOne(ctx, set, m); err != nil {
			return err
		}
	}

	slog.Info("migration set complete",
		"extension", set.extensionID,
		"applied", len(pending),
		"skipped", len(applied),
	)
	return nil
}

func (r *Runner) ensureHistoryTable(ctx context.Context, set setEntry) error {
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version    BIGINT PRIMARY KEY,
			filename   TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, set.table))
	if err != nil {
		return fmt.Errorf("create history table %s: %w", set.table, err)
	}
	return nil
}

func (r *Runner) appliedVersions(ctx context.Context, set setEntry) (map[int64]bool, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf("SELECT version FROM %s", set.table))
	if err != nil {
		return nil, fmt.Errorf("read history %s: %w", set.table, err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func (r *Runner) applyOne(ctx context.Context, set setEntry, m migrationFile) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.Content); err != nil {
		return fmt.Errorf("apply %s: %w", m.Filename, err)
	}

	_, err = tx.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s (version, filename) VALUES ($1, $2)", set.table),
		m.Version, m.Filename)
	if err != nil {
		return fmt.Errorf("record %s: %w", m.Filename, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	slog.Info("migration applied",
		"extension", set.extensionID,
		"version", m.Version,
		"file", m.Filename,
	)
	return nil
}

// parseVersion extracts the integer version from a migration filename.
// Expects format: V001__description.sql
func parseVersion(filename string) (int64, error) {
	matches := versionPattern.FindStringSubmatch(filename)
	if matches == nil {
		return 0, fmt.Errorf("not a migration file: %s (expected V<NNN>__name.sql)", filename)
	}
	return strconv.ParseInt(matches[1], 10, 64)
}

// readMigrationFiles reads all .sql files from the set's embedded FS.
func readMigrationFiles(set setEntry) ([]migrationFile, error) {
	entries, err := fs.ReadDir(set.fs, set.dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", set.dir, err)
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ver, err := parseVersion(entry.Name())
		if err != nil {
			continue // skip non-migration files
		}
		content, err := fs.ReadFile(set.fs, set.dir+"/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		files = append(files, migrationFile{
			Version:  ver,
			Filename: entry.Name(),
			Content:  string(content),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})

	return files, nil
}

// pendingFiles filters out already-applied migrations.
func pendingFiles(files []migrationFile, applied map[int64]bool) []migrationFile {
	var pending []migrationFile
	for _, f := range files {
		if !applied[f.Version] {
			pending = append(pending, f)
		}
	}
	return pending
}
```

- [ ] **Step 4: Run logic tests to verify they pass**

Run: `go test ./platform/migration/ -run "TestParse|TestSort|TestPending" -v`
Expected: PASS

- [ ] **Step 5: Add Testcontainers dependency**

Run: `go get github.com/testcontainers/testcontainers-go`

- [ ] **Step 6: Write the DB integration test**

```go
// platform/migration/runner_db_test.go
package migration

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

func TestRunnerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	// Create a fake FS with two migrations
	fakeFS := fstest.MapFS{
		"migrations/V001__create_table.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE test_items (id SERIAL PRIMARY KEY, name TEXT NOT NULL);`),
		},
		"migrations/V002__add_column.sql": &fstest.MapFile{
			Data: []byte(`ALTER TABLE test_items ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();`),
		},
	}

	sets := []platform.MigrationSet{{
		ExtensionID: "test-ext",
		FS:          fakeFS,
		Directory:   "migrations",
		Table:       "test_schema_migrations",
	}}

	// First run: both migrations applied
	runner := NewRunner(pool, sets)
	err = runner.Run(ctx)
	require.NoError(t, err)

	// Verify tables exist
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Verify history table recorded both
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both migrations should be recorded")

	// Second run: no-op (both already applied)
	err = runner.Run(ctx)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "re-run should not add new records")
}

func TestRunnerIsolatesExtensionHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	coreFS := fstest.MapFS{
		"migrations/V001__core.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE core_items (id SERIAL PRIMARY KEY);`),
		},
	}
	reportsFS := fstest.MapFS{
		"migrations/V001__reports.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE reports_items (id SERIAL PRIMARY KEY);`),
		},
	}

	sets := []platform.MigrationSet{
		{ExtensionID: "reports", FS: reportsFS, Directory: "migrations", Table: "schema_migrations_reports"},
		{ExtensionID: "platform-core", FS: coreFS, Directory: "migrations", Table: "schema_migrations_core"},
	}

	runner := NewRunner(pool, sets)
	err = runner.Run(ctx)
	require.NoError(t, err)

	// Verify isolated history tables
	var coreCount, reportsCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations_core").Scan(&coreCount)
	require.NoError(t, err)
	assert.Equal(t, 1, coreCount)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations_reports").Scan(&reportsCount)
	require.NoError(t, err)
	assert.Equal(t, 1, reportsCount)

	// Verify platform-core ran first (its table exists)
	var tableExists bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT FROM pg_tables WHERE tablename = 'core_items')").Scan(&tableExists)
	require.NoError(t, err)
	assert.True(t, tableExists, "core_items table should exist")

	fmt.Println("Isolated extension history verified")
}
```

- [ ] **Step 7: Run DB tests**

Run: `go test ./platform/migration/ -v -timeout 120s`
Expected: All tests PASS (DB tests will start Postgres containers; requires Docker/Podman running).

- [ ] **Step 8: Commit**

```bash
git add platform/migration/ go.mod go.sum
git commit -m "feat: add versioned migration runner with per-extension history (Testcontainers)"
```

---

## Task 8: Make existing migrations idempotent and move to core extension

**Files:**
- Modify: `migrations/V1__initial_schema.sql` → `internal/platform/core/migrations/V001__initial_schema.sql`
- Modify: `migrations/V2__user_profile_enhancements.sql` → `internal/platform/core/migrations/V002__user_profile_enhancements.sql`
- Modify: `migrations/V3__sessions_table.sql` → `internal/platform/core/migrations/V003__sessions_table.sql`
- Modify: `migrations/V4__user_preferences.sql` → `internal/platform/core/migrations/V004__user_preferences.sql`
- Create: `internal/platform/core/migrations.go`

- [ ] **Step 1: Move and rename the migration files**

Move the four SQL files from `migrations/` to `internal/platform/core/migrations/`, renaming with zero-padded version numbers:

```bash
mkdir -p internal/platform/core/migrations
cp migrations/V1__initial_schema.sql internal/platform/core/migrations/V001__initial_schema.sql
cp migrations/V2__user_profile_enhancements.sql internal/platform/core/migrations/V002__user_profile_enhancements.sql
cp migrations/V3__sessions_table.sql internal/platform/core/migrations/V003__sessions_table.sql
cp migrations/V4__user_preferences.sql internal/platform/core/migrations/V004__user_preferences.sql
```

- [ ] **Step 2: Make V001 idempotent**

In `internal/platform/core/migrations/V001__initial_schema.sql`, change every `CREATE TABLE ` to `CREATE TABLE IF NOT EXISTS ` and every `CREATE INDEX` / `CREATE UNIQUE INDEX` to include `IF NOT EXISTS`. This is a mechanical find-and-replace within the file.

Specifically:
- `CREATE TABLE plt_messages (` → `CREATE TABLE IF NOT EXISTS plt_messages (`
- Repeat for all 14 tables (plt_messages, plt_sync_state, plt_outbox, plt_users, plt_contacts, plt_contact_emails, plt_contact_phones, plt_contact_socials, plt_audit_log, plt_password_reset_tokens, plt_api_keys, plt_oauth_connections, plt_device_tokens, plt_notifications)
- `CREATE INDEX` → `CREATE INDEX IF NOT EXISTS`
- `CREATE UNIQUE INDEX` → `CREATE UNIQUE INDEX IF NOT EXISTS`

- [ ] **Step 3: Verify V002-V004 are already idempotent**

V002 and V004 use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (already idempotent). V003 (`CREATE TABLE plt_sessions`) needs the same `IF NOT EXISTS` treatment as V001. Check and fix:

- `CREATE TABLE plt_sessions (` → `CREATE TABLE IF NOT EXISTS plt_sessions (`
- Any indexes in V003 → add `IF NOT EXISTS`

- [ ] **Step 4: Create the embed file**

```go
// internal/platform/core/migrations.go
package core

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./internal/platform/core/`
Expected: Compiles without error.

- [ ] **Step 6: Delete the old migrations directory**

```bash
rm -rf migrations/
```

- [ ] **Step 7: Commit**

```bash
git add internal/platform/core/migrations/ internal/platform/core/migrations.go
git rm -r migrations/  # if not already removed
git commit -m "refactor: move migrations to core extension, make idempotent (IF NOT EXISTS)"
```

---

## Task 9: Core extension — wrap existing handlers

**Files:**
- Create: `internal/platform/core/core.go`
- Create: `internal/platform/core/contribute.go`
- Test: `internal/platform/core/contribute_test.go`

This is the largest mechanical task: mapping every existing route from `main.go`'s direct registration into the registry. The handler structs themselves don't change — only how their routes are registered.

- [ ] **Step 1: Write the failing test**

```go
// internal/platform/core/contribute_test.go
package core

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

// stubBundle is a minimal Bundle for testing the Contribute mapping.
// It uses http.HandlerFunc stubs since we only test route registration, not behavior.
type stubBundle struct{}

func TestCoreContributesAllRouteGroups(t *testing.T) {
	ext := NewExtension(stubBundle{})

	ctx := platform.NewContributionContext(ext.Manifest().ID)
	err := ext.Contribute(ctx)
	require.NoError(t, err)

	routes := ctx.Routes.All()

	// Should have routes in every group
	groups := map[platform.RouteGroup]bool{}
	for _, r := range routes {
		groups[r.Group] = true
	}

	assert.True(t, groups[platform.GroupPublicUI], "should have public UI routes (login)")
	assert.True(t, groups[platform.GroupProtectedUI], "should have protected UI routes")
	assert.True(t, groups[platform.GroupAPI], "should have API routes")
	assert.True(t, groups[platform.GroupAdmin], "should have admin routes")
}

func TestCoreManifest(t *testing.T) {
	ext := NewExtension(stubBundle{})
	m := ext.Manifest()

	assert.Equal(t, "platform-core", m.ID)
	assert.Equal(t, platform.FullPlatform, m.Mode)
	assert.NotEmpty(t, m.Ownership.UI)
	assert.NotEmpty(t, m.Ownership.API)
	assert.NotEmpty(t, m.Ownership.Admin)
}

func TestCoreNavigationItems(t *testing.T) {
	ext := NewExtension(stubBundle{})
	ctx := platform.NewContributionContext(ext.Manifest().ID)
	err := ext.Contribute(ctx)
	require.NoError(t, err)

	nav := ctx.Navigation.Items()
	labels := make([]string, len(nav))
	for i, item := range nav {
		labels[i] = item.Label
	}

	assert.Contains(t, labels, "Home")
	assert.Contains(t, labels, "Contacts")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/core/ -v`
Expected: FAIL — package doesn't compile.

- [ ] **Step 3: Write the Bundle type and Extension**

The `Bundle` groups all existing handlers so the core extension can reference them. Since we don't want to change the existing handler structs, `Bundle` is a simple struct of pointers.

```go
// internal/platform/core/core.go
package core

import (
	"net/http"

	platform "github.com/rygel/gouterstellar-platform/platform"
	"github.com/rygel/gouterstellar-platform/internal/platform/core/migrations"
)

// Bundle holds all the platform's internal HTTP handlers.
// It's the bridge between the existing handler structs and the core extension.
// Each field is a handler method that the Contribute method maps to registry calls.
type Bundle struct {
	// PublicUI handlers
	AuthShowLogin        http.HandlerFunc
	AuthHandleLogin      http.HandlerFunc
	AuthHandleRegister   http.HandlerFunc
	AuthHandleLogout     http.HandlerFunc
	AuthShowChangePwd    http.HandlerFunc
	AuthHandleChangePwd  http.HandlerFunc
	AuthShowReset        http.HandlerFunc
	AuthHandleReset      http.HandlerFunc
	OAuthRedirect        http.HandlerFunc
	OAuthCallback        http.HandlerFunc
	OAuthCallbackPost    http.HandlerFunc

	// ProtectedUI handlers
	HomeShow             http.HandlerFunc
	ContactsList         http.HandlerFunc
	ContactsDetail       http.HandlerFunc
	ContactsCreate       http.HandlerFunc
	ContactsUpdate       http.HandlerFunc
	ContactsDelete       http.HandlerFunc
	SearchSearch         http.HandlerFunc
	SettingsShow         http.HandlerFunc
	SettingsProfile      http.HandlerFunc
	SettingsPassword     http.HandlerFunc
	SettingsPreferences  http.HandlerFunc
	SettingsCreateAPIKey http.HandlerFunc
	SettingsDeleteAPIKey http.HandlerFunc
	SettingsNotifPrefs   http.HandlerFunc
	NotifsList           http.HandlerFunc
	NotifsMarkRead       http.HandlerFunc
	NotifsMarkAllRead    http.HandlerFunc
	NotifsDelete         http.HandlerFunc
	ComponentsMsgList    http.HandlerFunc
	ComponentsContactList http.HandlerFunc
	SyncWebSocket        http.HandlerFunc
	ErrorNotFound        http.HandlerFunc

	// API handlers
	SyncPullMessages     http.HandlerFunc
	SyncPushMessages     http.HandlerFunc
	SyncPullContacts     http.HandlerFunc
	SyncPushContacts     http.HandlerFunc
	AuthAPILogin         http.HandlerFunc
	AuthAPIToken         http.HandlerFunc
	AuthAPIRegister      http.HandlerFunc
	AuthAPIChangePwd     http.HandlerFunc
	AuthAPIResetReq      http.HandlerFunc
	AuthAPIConfirmReset  http.HandlerFunc
	AuthAPILogout        http.HandlerFunc
	AuthAPIGetProfile    http.HandlerFunc
	AuthAPIUpdateProfile http.HandlerFunc
	AuthAPINotifPrefs    http.HandlerFunc
	AuthAPIDeleteAccount http.HandlerFunc
	AuthAPICreateAPIKey  http.HandlerFunc
	AuthAPIListAPIKeys   http.HandlerFunc
	AuthAPIDeleteAPIKey  http.HandlerFunc
	UserAPIListUsers     http.HandlerFunc
	UserAPICountUsers    http.HandlerFunc
	UserAPISetEnabled    http.HandlerFunc
	UserAPISetRole       http.HandlerFunc
	UserAPIExportUsers   http.HandlerFunc
	UserAPIExportAudit   http.HandlerFunc
	NotifAPIList         http.HandlerFunc
	NotifAPIUnreadCount  http.HandlerFunc
	NotifAPIMarkRead     http.HandlerFunc
	NotifAPIMarkAllRead  http.HandlerFunc
	NotifAPIDelete       http.HandlerFunc
	DeviceAPIRegister    http.HandlerFunc
	DeviceAPIUnregister  http.HandlerFunc

	// Admin handlers
	AdminListUsers       http.HandlerFunc
	AdminSetEnabled      http.HandlerFunc
	AdminSetRole         http.HandlerFunc
	AdminExportUsers     http.HandlerFunc
	AdminShowAudit       http.HandlerFunc
	AdminExportAudit     http.HandlerFunc
	DevDashboard         http.HandlerFunc
	DevProcessOutbox     http.HandlerFunc
	DevCleanupSessions   http.HandlerFunc
	DevInvalidateCache   http.HandlerFunc

	// Health/metrics
	Health http.HandlerFunc

	// Dev mode flag
	DevDashboardEnabled bool
}

// Extension is the core platform extension. It wraps all built-in handlers
// and contributes their routes through the registry.
type Extension struct {
	bundle *Bundle
}

// NewExtension creates the core extension from a populated Bundle.
func NewExtension(b Bundle) *Extension {
	return &Extension{bundle: &b}
}

func (e *Extension) Manifest() platform.Manifest {
	return platform.Manifest{
		ID:    "platform-core",
		Label: "Platform Core",
		Mode:  platform.FullPlatform,
		Ownership: platform.RouteOwnership{
			UI: []string{
				"/", "/auth", "/contacts", "/search", "/settings",
				"/notifications", "/components", "/ws",
			},
			API:   []string{"/api/v1"},
			Admin: []string{"/admin", "/dev"},
			Assets: []string{"/static"},
		},
		Migrations: []platform.MigrationSet{
			{
				ExtensionID: "platform-core",
				FS:          migrations.Migrations,
				Directory:   "migrations",
				Table:       "schema_migrations_core",
			},
		},
	}
}
```

Note: the import `migrations` references the `internal/platform/core/migrations.go` embed file. The package within `internal/platform/core/` that holds the embed must be the same package (`core`), or a sub-package. Since `//go:embed` and the `embed` package must be in a variable, and the embed file is in the same package, adjust:

```go
// Remove the separate migrations.go import; instead embed directly in core.go:
//go:embed migrations/*.sql
var coreMigrations embed.FS
```

And reference `coreMigrations` in the Manifest instead of `migrations.Migrations`. Delete the separate `migrations.go` file from Task 8 to avoid a duplicate — fold the embed into `core.go`.

- [ ] **Step 4: Write the Contribute method (route mapping)**

```go
// internal/platform/core/contribute.go
package core

import (
	"net/http"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

func (e *Extension) Contribute(ctx *platform.ContributionContext) error {
	b := e.bundle

	// --- Public UI (no auth required) ---
	ctx.Routes.Public(http.MethodGet, "/auth", "Login page", http.HandlerFunc(b.AuthShowLogin))
	ctx.Routes.Public(http.MethodPost, "/auth/login", "Handle login", http.HandlerFunc(b.AuthHandleLogin))
	ctx.Routes.Public(http.MethodPost, "/auth/register", "Handle registration", http.HandlerFunc(b.AuthHandleRegister))
	ctx.Routes.Public(http.MethodPost, "/auth/logout", "Handle logout", http.HandlerFunc(b.AuthHandleLogout))
	ctx.Routes.Public(http.MethodGet, "/auth/change-password", "Change password page", http.HandlerFunc(b.AuthShowChangePwd))
	ctx.Routes.Public(http.MethodPost, "/auth/change-password", "Handle password change", http.HandlerFunc(b.AuthHandleChangePwd))
	ctx.Routes.Public(http.MethodGet, "/auth/reset", "Reset password page", http.HandlerFunc(b.AuthShowReset))
	ctx.Routes.Public(http.MethodPost, "/auth/reset", "Handle password reset", http.HandlerFunc(b.AuthHandleReset))
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}", "OAuth redirect", http.HandlerFunc(b.OAuthRedirect))
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}/callback", "OAuth callback", http.HandlerFunc(b.OAuthCallback))
	ctx.Routes.Public(http.MethodPost, "/auth/oauth/{provider}/callback", "OAuth callback POST", http.HandlerFunc(b.OAuthCallbackPost))

	// --- Protected UI (auth required) ---
	ctx.Routes.Protected(http.MethodGet, "/", "Home dashboard", http.HandlerFunc(b.HomeShow))
	ctx.Routes.Protected(http.MethodGet, "/contacts", "Contacts list", http.HandlerFunc(b.ContactsList))
	ctx.Routes.Protected(http.MethodGet, "/contacts/{syncId}", "Contact detail", http.HandlerFunc(b.ContactsDetail))
	ctx.Routes.Protected(http.MethodPost, "/contacts/create", "Create contact", http.HandlerFunc(b.ContactsCreate))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/update", "Update contact", http.HandlerFunc(b.ContactsUpdate))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/delete", "Delete contact", http.HandlerFunc(b.ContactsDelete))
	ctx.Routes.Protected(http.MethodGet, "/search", "Search", http.HandlerFunc(b.SearchSearch))
	ctx.Routes.Protected(http.MethodGet, "/settings", "Settings page", http.HandlerFunc(b.SettingsShow))
	ctx.Routes.Protected(http.MethodPost, "/settings/profile", "Update profile", http.HandlerFunc(b.SettingsProfile))
	ctx.Routes.Protected(http.MethodPost, "/settings/password", "Change password", http.HandlerFunc(b.SettingsPassword))
	ctx.Routes.Protected(http.MethodPost, "/settings/preferences", "Update preferences", http.HandlerFunc(b.SettingsPreferences))
	ctx.Routes.Protected(http.MethodPost, "/settings/api-keys", "Create API key", http.HandlerFunc(b.SettingsCreateAPIKey))
	ctx.Routes.Protected(http.MethodPost, "/settings/api-keys/{id}/delete", "Delete API key", http.HandlerFunc(b.SettingsDeleteAPIKey))
	ctx.Routes.Protected(http.MethodPost, "/settings/notifications", "Update notification prefs", http.HandlerFunc(b.SettingsNotifPrefs))
	ctx.Routes.Protected(http.MethodGet, "/notifications", "Notifications list", http.HandlerFunc(b.NotifsList))
	ctx.Routes.Protected(http.MethodPost, "/notifications/{id}/read", "Mark notification read", http.HandlerFunc(b.NotifsMarkRead))
	ctx.Routes.Protected(http.MethodPost, "/notifications/read-all", "Mark all read", http.HandlerFunc(b.NotifsMarkAllRead))
	ctx.Routes.Protected(http.MethodPost, "/notifications/{id}/delete", "Delete notification", http.HandlerFunc(b.NotifsDelete))
	ctx.Routes.Protected(http.MethodGet, "/components/message-list", "Message list partial", http.HandlerFunc(b.ComponentsMsgList))
	ctx.Routes.Protected(http.MethodGet, "/components/contact-list", "Contact list partial", http.HandlerFunc(b.ComponentsContactList))
	ctx.Routes.Protected(http.MethodGet, "/ws/sync", "WebSocket sync", http.HandlerFunc(b.SyncWebSocket))

	// --- API (bearer auth applied by builder) ---
	ctx.Routes.API(http.MethodGet, "/api/v1/sync", "Pull message changes", http.HandlerFunc(b.SyncPullMessages))
	ctx.Routes.API(http.MethodPost, "/api/v1/sync", "Push message changes", http.HandlerFunc(b.SyncPushMessages))
	ctx.Routes.API(http.MethodGet, "/api/v1/sync/contacts", "Pull contact changes", http.HandlerFunc(b.SyncPullContacts))
	ctx.Routes.API(http.MethodPost, "/api/v1/sync/contacts", "Push contact changes", http.HandlerFunc(b.SyncPushContacts))

	ctx.Routes.API(http.MethodPost, "/api/v1/auth/login", "API login", http.HandlerFunc(b.AuthAPILogin))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/token", "Issue token", http.HandlerFunc(b.AuthAPIToken))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/register", "API register", http.HandlerFunc(b.AuthAPIRegister))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/change-password", "API change password", http.HandlerFunc(b.AuthAPIChangePwd))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/reset-password", "Request password reset", http.HandlerFunc(b.AuthAPIResetReq))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/confirm-reset", "Confirm password reset", http.HandlerFunc(b.AuthAPIConfirmReset))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/logout", "API logout", http.HandlerFunc(b.AuthAPILogout))
	ctx.Routes.API(http.MethodGet, "/api/v1/auth/profile", "Get profile", http.HandlerFunc(b.AuthAPIGetProfile))
	ctx.Routes.API(http.MethodPut, "/api/v1/auth/profile", "Update profile", http.HandlerFunc(b.AuthAPIUpdateProfile))
	ctx.Routes.API(http.MethodPut, "/api/v1/auth/notification-preferences", "Update notif prefs", http.HandlerFunc(b.AuthAPINotifPrefs))
	ctx.Routes.API(http.MethodDelete, "/api/v1/auth/account", "Delete account", http.HandlerFunc(b.AuthAPIDeleteAccount))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/api-keys", "Create API key", http.HandlerFunc(b.AuthAPICreateAPIKey))
	ctx.Routes.API(http.MethodGet, "/api/v1/auth/api-keys", "List API keys", http.HandlerFunc(b.AuthAPIListAPIKeys))
	ctx.Routes.API(http.MethodDelete, "/api/v1/auth/api-keys/{id}", "Delete API key", http.HandlerFunc(b.AuthAPIDeleteAPIKey))

	ctx.Routes.API(http.MethodGet, "/api/v1/users", "List users", http.HandlerFunc(b.UserAPIListUsers))
	ctx.Routes.API(http.MethodGet, "/api/v1/users/count", "Count users", http.HandlerFunc(b.UserAPICountUsers))
	ctx.Routes.API(http.MethodPut, "/api/v1/users/{id}/enabled", "Set user enabled", http.HandlerFunc(b.UserAPISetEnabled))
	ctx.Routes.API(http.MethodPut, "/api/v1/users/{id}/role", "Set user role", http.HandlerFunc(b.UserAPISetRole))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/users/export", "Export users CSV", http.HandlerFunc(b.UserAPIExportUsers))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/audit/export", "Export audit CSV", http.HandlerFunc(b.UserAPIExportAudit))

	ctx.Routes.API(http.MethodGet, "/api/v1/notifications", "List notifications", http.HandlerFunc(b.NotifAPIList))
	ctx.Routes.API(http.MethodGet, "/api/v1/notifications/unread-count", "Unread count", http.HandlerFunc(b.NotifAPIUnreadCount))
	ctx.Routes.API(http.MethodPut, "/api/v1/notifications/{id}/read", "Mark read", http.HandlerFunc(b.NotifAPIMarkRead))
	ctx.Routes.API(http.MethodPut, "/api/v1/notifications/read-all", "Mark all read", http.HandlerFunc(b.NotifAPIMarkAllRead))
	ctx.Routes.API(http.MethodDelete, "/api/v1/notifications/{id}", "Delete notification", http.HandlerFunc(b.NotifAPIDelete))

	ctx.Routes.API(http.MethodPost, "/api/v1/devices/register", "Register device", http.HandlerFunc(b.DeviceAPIRegister))
	ctx.Routes.API(http.MethodDelete, "/api/v1/devices/{id}", "Unregister device", http.HandlerFunc(b.DeviceAPIUnregister))

	// --- Admin ---
	ctx.Routes.Admin(http.MethodGet, "/admin/users", "User management", http.HandlerFunc(b.AdminListUsers))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/enabled", "Set user enabled", http.HandlerFunc(b.AdminSetEnabled))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/role", "Set user role", http.HandlerFunc(b.AdminSetRole))
	ctx.Routes.Admin(http.MethodGet, "/admin/users/export", "Export users", http.HandlerFunc(b.AdminExportUsers))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit", "Audit log", http.HandlerFunc(b.AdminShowAudit))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit/export", "Export audit", http.HandlerFunc(b.AdminExportAudit))

	if b.DevDashboardEnabled {
		ctx.Routes.Admin(http.MethodGet, "/dev/dashboard", "Dev dashboard", http.HandlerFunc(b.DevDashboard))
		ctx.Routes.Admin(http.MethodPost, "/dev/outbox/process", "Process outbox", http.HandlerFunc(b.DevProcessOutbox))
		ctx.Routes.Admin(http.MethodPost, "/dev/sessions/cleanup", "Cleanup sessions", http.HandlerFunc(b.DevCleanupSessions))
		ctx.Routes.Admin(http.MethodPost, "/dev/cache/invalidate", "Invalidate cache", http.HandlerFunc(b.DevInvalidateCache))
	}

	// --- Navigation ---
	ctx.Navigation.Add("Home", "/", "house")
	ctx.Navigation.Add("Contacts", "/contacts", "users")
	ctx.Navigation.Add("Search", "/search", "search")
	ctx.Navigation.Add("Settings", "/settings", "gear")
	ctx.Navigation.Add("Notifications", "/notifications", "bell")

	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/platform/core/ -v`
Expected: PASS (the stubBundle test uses zero-value Bundle — all HandlerFunc fields are nil, but the test only checks that routes are registered, not called).

Note: if nil `http.HandlerFunc` causes a panic when stored, change `stubBundle` to populate fields with `http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})`. Update the test:

```go
func TestCoreContributesAllRouteGroups(t *testing.T) {
	b := Bundle{
		AuthShowLogin: stub, HomeShow: stub, /* ... enough to cover each group */
		SyncPullMessages: stub, AdminListUsers: stub,
	}
	ext := NewExtension(b)
	// ...
}

var stub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
```

- [ ] **Step 6: Commit**

```bash
git add internal/platform/core/
git commit -m "feat: add core extension wrapping all existing handlers"
```

---

## Task 10: Update `wire.go` — remove plugin manager, add extension assembly

**Files:**
- Modify: `internal/wire/wire.go`

- [ ] **Step 1: Remove plugin imports and PluginManager**

In `internal/wire/wire.go`:
1. Remove the import `"github.com/rygel/gouterstellar-platform/pkg/plugin"`
2. Remove the `PluginManager *plugin.PluginManager` field from the `App` struct
3. Remove `pluginManager := plugin.NewPluginManager()` (line 181)
4. Remove `PluginManager: pluginManager,` from the return struct

- [ ] **Step 2: Add the core Bundle builder**

Add a method that constructs a `core.Bundle` from the existing handlers, mapping each handler's methods to the Bundle's `http.HandlerFunc` fields. Add this to `wire.go`:

```go
// Add imports:
//   "github.com/rygel/gouterstellar-platform/internal/platform/core"
//   extplatform "github.com/rygel/gouterstellar-platform/internal/platform"

// Add to App struct:
//   Extensions []platform.Extension
//   CoreBundle core.Bundle

// Add a function that builds the core Bundle from handlers:
func buildCoreBundle(app *App, cfg *config.Config) core.Bundle {
	return core.Bundle{
		// PublicUI
		AuthShowLogin:       app.AuthHandler.ShowLogin,
		AuthHandleLogin:     app.AuthHandler.HandleLogin,
		AuthHandleRegister:  app.AuthHandler.HandleRegister,
		AuthHandleLogout:    app.AuthHandler.HandleLogout,
		AuthShowChangePwd:   app.AuthHandler.ShowChangePassword,
		AuthHandleChangePwd: app.AuthHandler.HandleChangePassword,
		AuthShowReset:       app.AuthHandler.ShowResetPassword,
		AuthHandleReset:     app.AuthHandler.HandleResetPassword,
		OAuthRedirect:       app.OAuthHandler.Redirect,
		OAuthCallback:       app.OAuthHandler.Callback,
		OAuthCallbackPost:   app.OAuthHandler.CallbackPost,

		// ProtectedUI
		HomeShow:             app.HomeHandler.Show,
		ContactsList:         app.ContactsHandler.List,
		ContactsDetail:       app.ContactsHandler.Detail,
		ContactsCreate:       app.ContactsHandler.Create,
		ContactsUpdate:       app.ContactsHandler.Update,
		ContactsDelete:       app.ContactsHandler.Delete,
		SearchSearch:         app.SearchHandler.Search,
		SettingsShow:         app.SettingsHandler.Show,
		SettingsProfile:      app.SettingsHandler.UpdateProfile,
		SettingsPassword:     app.SettingsHandler.ChangePassword,
		SettingsPreferences:  app.SettingsHandler.UpdatePreferences,
		SettingsCreateAPIKey: app.SettingsHandler.CreateApiKey,
		SettingsDeleteAPIKey: app.SettingsHandler.DeleteApiKey,
		SettingsNotifPrefs:   app.SettingsHandler.UpdateNotificationPrefs,
		NotifsList:           app.NotificationsHandler.List,
		NotifsMarkRead:       app.NotificationsHandler.MarkRead,
		NotifsMarkAllRead:    app.NotificationsHandler.MarkAllRead,
		NotifsDelete:         app.NotificationsHandler.Delete,
		ComponentsMsgList:    app.ComponentsHandler.MessageList,
		ComponentsContactList: app.ComponentsHandler.ContactList,
		SyncWebSocket:        app.SyncWebSocket.Handle,

		// API
		SyncPullMessages:     app.SyncAPI.PullMessages,
		SyncPushMessages:     app.SyncAPI.PushMessages,
		SyncPullContacts:     app.SyncAPI.PullContacts,
		SyncPushContacts:     app.SyncAPI.PushContacts,
		AuthAPILogin:         app.AuthAPI.Login,
		AuthAPIToken:         app.AuthAPI.IssueToken,
		AuthAPIRegister:      app.AuthAPI.Register,
		AuthAPIChangePwd:     app.AuthAPI.ChangePassword,
		AuthAPIResetReq:      app.AuthAPI.RequestPasswordReset,
		AuthAPIConfirmReset:  app.AuthAPI.ConfirmPasswordReset,
		AuthAPILogout:        app.AuthAPI.Logout,
		AuthAPIGetProfile:    app.AuthAPI.GetProfile,
		AuthAPIUpdateProfile: app.AuthAPI.UpdateProfile,
		AuthAPINotifPrefs:    app.AuthAPI.UpdateNotificationPreferences,
		AuthAPIDeleteAccount: app.AuthAPI.DeleteAccount,
		AuthAPICreateAPIKey:  app.AuthAPI.CreateApiKey,
		AuthAPIListAPIKeys:   app.AuthAPI.ListApiKeys,
		AuthAPIDeleteAPIKey:  app.AuthAPI.DeleteApiKey,
		UserAPIListUsers:     app.UserAdminAPI.ListUsers,
		UserAPICountUsers:    app.UserAdminAPI.CountUsers,
		UserAPISetEnabled:    app.UserAdminAPI.SetEnabled,
		UserAPISetRole:       app.UserAdminAPI.SetRole,
		UserAPIExportUsers:   app.UserAdminAPI.ExportUsersCSV,
		UserAPIExportAudit:   app.UserAdminAPI.ExportAuditCSV,
		NotifAPIList:         app.NotificationAPI.List,
		NotifAPIUnreadCount:  app.NotificationAPI.UnreadCount,
		NotifAPIMarkRead:     app.NotificationAPI.MarkRead,
		NotifAPIMarkAllRead:  app.NotificationAPI.MarkAllRead,
		NotifAPIDelete:       app.NotificationAPI.Delete,
		DeviceAPIRegister:    app.DeviceRegistrationAPI.Register,
		DeviceAPIUnregister:  app.DeviceRegistrationAPI.Unregister,

		// Admin
		AdminListUsers:     app.UserAdminHandler.ListUsers,
		AdminSetEnabled:    app.UserAdminHandler.SetEnabled,
		AdminSetRole:       app.UserAdminHandler.SetRole,
		AdminExportUsers:   app.UserAdminHandler.ExportUsers,
		AdminShowAudit:     app.UserAdminHandler.ShowAudit,
		AdminExportAudit:   app.UserAdminHandler.ExportAudit,
		DevDashboard:       app.DevDashboardHandler.Show,
		DevProcessOutbox:   app.DevDashboardHandler.ProcessOutbox,
		DevCleanupSessions: app.DevDashboardHandler.CleanupSessions,
		DevInvalidateCache: app.DevDashboardHandler.InvalidateCache,

		DevDashboardEnabled: cfg.DevDashboardEnabled,
	}
}
```

Note: some of these method references (e.g. `app.AuthHandler.ShowLogin`) are `http.HandlerFunc` method values — Go will automatically promote the method to `http.HandlerFunc` since the handler methods have the signature `func(w http.ResponseWriter, r *http.Request)`. Verify each compiles; if any handler method has a different signature (e.g. returns an error), wrap it in a closure.

- [ ] **Step 3: Add the ServiceBag builder**

In `wire.go`, after the handler construction, add:

```go
svcBag := extplatform.BuildServiceBag(messageSvc, contactSvc, securitySvc)
```

And store it on `App`:
```go
// In App struct: ServiceBag extplatform.ServiceBag
// In return: ServiceBag: svcBag,
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/wire/`
Expected: Compiles without error.

- [ ] **Step 5: Commit**

```bash
git add internal/wire/wire.go
git commit -m "refactor: remove plugin manager, add core bundle and service bag to wire"
```

---

## Task 11: Update `main.go` — use `platform.NewHandler`

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Rewrite main.go to use the assembly path**

Replace the route registration block (lines 48-118) with the new assembly. The middleware chain is passed into Options. Background goroutines stay.

```go
// cmd/server/main.go — rewrite the router section:

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

	if err := pool.Ping(ctx); err != nil {
		slog.Error("Database ping failed", "error", err)
		os.Exit(1)
	}

	templateFS := web.TemplateFS()
	app := wire.Wire(cfg, pool, templateFS)

	// Build the core bundle from the assembled handlers.
	coreBundle := wire.BuildCoreBundle(app, cfg)
	coreExt := core.NewExtension(coreBundle)

	// Build the middleware chain (same order as before).
	middlewareChain := []func(http.Handler) http.Handler{
		chimw.RequestID,
		chimw.RealIP,
		chimw.Recoverer,
		chimw.Timeout(60 * time.Second),
		cors.Handler(cors.Options{
			AllowedOrigins:   strings.Split(cfg.CORSOrigins, ","),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}),
		filter.SecurityHeaders(cfg.CSPPolicy, cfg.SessionCookieSecure),
		filter.DevAutoLogin(func() uuid.UUID {
			return app.SecurityService.DevAdminID(ctx)
		}, app.SecurityService, cfg.DevMode),
		filter.RateLimiter(10, 20),
		filter.CSRF(cfg.CSRFEnabled),
		filter.Session(app.SecurityService, cfg.SessionCookieSecure),
		filter.Logging(),
	}

	// Assemble via the extension model.
	handler, err := platform.NewHandler(platform.Options{
		Mode: platform.FullPlatform,
		Extensions: []platform.Extension{
			coreExt,
		},
		Services:        app.ServiceBag,
		MiddlewareChain: middlewareChain,
	})
	if err != nil {
		slog.Error("Platform assembly failed", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      handler,
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

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := app.OutboxProcessor.ProcessPending(context.Background()); err != nil {
				slog.Error("Outbox processing failed", "error", err)
			}
			app.ActivityUpdater.Flush()
			if err := app.SecurityService.DeleteExpiredSessions(context.Background()); err != nil {
				slog.Error("Session cleanup failed", "error", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	fmt.Println("Server stopped")
}
```

Note: the health check (`/health`), metrics (`/metrics`), and static file server (`/static/*`) are currently registered directly. In the new model, these should be contributed by the core extension. Add them to the `Bundle` struct:

```go
// In core.Bundle: add:
Health  http.HandlerFunc
Metrics http.Handler
Static  http.Handler

// In core.Contribute: add:
// Health is a special route — it's neither UI nor API. Register as Public.
ctx.Routes.Public(http.MethodGet, "/health", "Health check", http.HandlerFunc(b.Health))
ctx.Routes.Assets("/static/*", b.Static)
// Metrics: register as API group (no auth, but JSON content)
ctx.Routes.API(http.MethodGet, "/metrics", "Prometheus metrics", b.Metrics)
```

Wait — `Assets` takes a pattern and handler, but `/static/*` is served by `http.FileServer` with `StripPrefix`. Register it directly on the router in the builder or add a dedicated asset handler to the Bundle. The simplest approach: register `/static/*` as an asset route in the core extension's Contribute using the AssetRegistry or a direct Protected group call.

For the PoC, add health/metrics/static as direct registrations in `buildRoutes` before the extension routes — or better, handle them in the core extension. Since `/health` and `/metrics` have special handler types (one is `http.HandlerFunc`, one is `http.Handler` from promhttp), store them on the Bundle and register in Contribute.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/server/`
Expected: Compiles without error. May require import adjustments.

- [ ] **Step 3: Run existing tests to verify nothing is broken**

Run: `go test ./internal/... -v -timeout 60s`
Expected: All existing tests PASS (they don't test routing through main.go).

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: replace direct Chi wiring with platform.NewHandler assembly"
```

---

## Task 12: Reports extension — the reference extension

**Files:**
- Create: `extensions/reports/reports.go`
- Create: `extensions/reports/handlers.go`
- Create: `extensions/reports/migrations.go`
- Create: `extensions/reports/migrations/V001__reports_tables.sql`
- Test: `extensions/reports/reports_test.go`
- Test: `extensions/reports/reports_http_test.go`

- [ ] **Step 1: Write the contract test (Tier 1)**

```go
// extensions/reports/reports_test.go
package reports

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

type stubMessageCounter struct{ count int64 }

func (s stubMessageCounter) CountMessages(ctx context.Context) (int64, error) {
	return s.count, nil
}

func TestReportsContract(t *testing.T) {
	ext := New(stubMessageCounter{count: 42})

	diag, err := platform.CheckExtension(ext, platform.TestHostContext())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"GET /reports", "GET /api/v1/reports/summary"},
		diag.RoutePatterns(),
	)
	assert.Contains(t, diag.NavigationLabels(), "Reports")
}

func TestReportsManifest(t *testing.T) {
	ext := New(stubMessageCounter{})
	m := ext.Manifest()

	assert.Equal(t, "reports", m.ID)
	assert.Equal(t, platform.ExtensionHost, m.Mode)
	assert.NotEmpty(t, m.Ownership.UI)
	assert.NotEmpty(t, m.Ownership.API)
	require.Len(t, m.Migrations, 1)
	assert.Equal(t, "schema_migrations_reports", m.Migrations[0].Table)
}
```

- [ ] **Step 2: Run contract test to verify it fails**

Run: `go test ./extensions/reports/ -run TestReports -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Write the reports extension**

```go
// extensions/reports/reports.go
package reports

import (
	"embed"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Extension is the reports extension. It demonstrates the full extension model.
type Extension struct {
	messages platform.MessageCounter
}

// New creates the reports extension with the given message counter capability.
func New(messages platform.MessageCounter) *Extension {
	return &Extension{messages: messages}
}

func (e *Extension) Manifest() platform.Manifest {
	return platform.Manifest{
		ID:    "reports",
		Label: "Reports",
		Mode:  platform.ExtensionHost,
		Ownership: platform.RouteOwnership{
			UI:    []string{"/reports", "/extension/reports"},
			API:   []string{"/api/reports", "/api/v1/reports"},
			Admin: []string{"/admin/reports"},
		},
		Migrations: []platform.MigrationSet{{
			ExtensionID: "reports",
			FS:          migrationsFS,
			Directory:   "migrations",
			Table:       "schema_migrations_reports",
		}},
	}
}
```

```go
// extensions/reports/handlers.go
package reports

import (
	"encoding/json"
	"net/http"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

func (e *Extension) Contribute(ctx *platform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/reports", "Reports home", http.HandlerFunc(e.home))
	ctx.Routes.API(http.MethodGet, "/api/v1/reports/summary", "Message count summary", http.HandlerFunc(e.summary))
	ctx.Navigation.Add("Reports", "/reports", "bar-chart")
	return nil
}

func (e *Extension) home(w http.ResponseWriter, r *http.Request) {
	count, err := e.messages.CountMessages(r.Context())
	if err != nil {
		http.Error(w, "Failed to load report data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>Reports</title></head><body>"))
	_, _ = w.Write([]byte("<h1>Reports</h1>"))
	_, _ = w.Write([]byte("<p>Messages: "))
	_, _ = fmt.Fprintf(w, "%d", count)
	_, _ = w.Write([]byte("</p>"))
	_, _ = w.Write([]byte("</body></html>"))
}

func (e *Extension) summary(w http.ResponseWriter, r *http.Request) {
	count, err := e.messages.CountMessages(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "count failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message_count": count,
	})
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
```

Note: add `"fmt"` to the imports in handlers.go.

```go
// extensions/reports/migrations.go
package reports

// migrationsFS is embedded via the //go:embed directive in reports.go.
// This file exists to document that; the embed var is in reports.go.
```

```sql
-- extensions/reports/migrations/V001__reports_tables.sql
CREATE TABLE IF NOT EXISTS reports_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    captured_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_count BIGINT NOT NULL,
    contact_count BIGINT NOT NULL
);
```

- [ ] **Step 4: Run contract test to verify it passes**

Run: `go test ./extensions/reports/ -run TestReports -v`
Expected: PASS

- [ ] **Step 5: Write the HTTP test (Tier 2)**

```go
// extensions/reports/reports_http_test.go
package reports

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platform "github.com/rygel/gouterstellar-platform/platform"
)

func TestReportsHomeHTTP(t *testing.T) {
	ext := New(stubMessageCounter{count: 99})

	app, err := platform.NewTestApp(platform.TestOptions{
		Mode:       platform.FullPlatform,
		Extensions: []platform.Extension{ext},
	})
	require.NoError(t, err)
	t.Cleanup(app.Close)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Reports")
	assert.Contains(t, rec.Body.String(), "99")
}

func TestReportsSummaryAPI(t *testing.T) {
	ext := New(stubMessageCounter{count: 7})

	app, err := platform.NewTestApp(platform.TestOptions{
		Mode:       platform.FullPlatform,
		Extensions: []platform.Extension{ext},
	})
	require.NoError(t, err)
	t.Cleanup(app.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "message_count")
	assert.Contains(t, rec.Body.String(), "7")
}

func TestReportsRejectsRouteOutsideOwnership(t *testing.T) {
	// Reports doesn't own /settings — if it tried to register it,
	// validation should fail. This test verifies by checking that a
	// conflicting extension is rejected.
	ext := New(stubMessageCounter{})

	// Verify the manifest's ownership doesn't include /settings
	m := ext.Manifest()
	for _, prefix := range m.Ownership.UI {
		assert.NotEqual(t, "/settings", prefix)
	}
}
```

- [ ] **Step 6: Run HTTP tests to verify they pass**

Run: `go test ./extensions/reports/ -v`
Expected: All tests PASS.

- [ ] **Step 7: Add reports to the server's extension list**

In `cmd/server/main.go`, after creating `coreExt`, add:

```go
// Add import: "github.com/rygel/gouterstellar-platform/extensions/reports"

// After coreExt:
reportsExt := reports.New(app.ServiceBag.MessageCounter)

// In the Extensions slice:
Extensions: []platform.Extension{
    coreExt,
    reportsExt,
},
```

- [ ] **Step 8: Verify it compiles and builds**

Run: `go build ./cmd/server/`
Expected: Compiles without error.

- [ ] **Step 9: Commit**

```bash
git add extensions/
git commit -m "feat: add reports extension proving the extension model"
```

---

## Task 13: Delete `pkg/plugin/` and clean up

**Files:**
- Delete: `pkg/plugin/plugin.go`
- Delete: `pkg/plugin/manager.go`
- Delete: `pkg/plugin/manager_test.go`

- [ ] **Step 1: Delete the plugin package**

```bash
rm -rf pkg/plugin/
```

- [ ] **Step 2: Verify nothing else imports it**

Run: `grep -r "pkg/plugin" --include="*.go" .`
Expected: No results (wire.go was already cleaned in Task 10).

- [ ] **Step 3: Verify the project builds**

Run: `go build ./...`
Expected: Compiles without error.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: remove pkg/plugin/ — replaced by extension model"
```

---

## Task 14: Update `cmd/migrate` to use the new runner

**Files:**
- Modify: `cmd/migrate/main.go`

- [ ] **Step 1: Rewrite the migrate binary**

```go
// cmd/migrate/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	platform "github.com/rygel/gouterstellar-platform/platform"
	"github.com/rygel/gouterstellar-platform/platform/migration"
	"github.com/rygel/gouterstellar-platform/internal/platform/core"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if len(os.Args) > 1 {
		dbURL = os.Args[1]
	}
	if dbURL == "" {
		slog.Error("DATABASE_URL env var or CLI argument required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Build the migration sets from the same extension list the server uses.
	// This ensures cmd/migrate and cmd/server never disagree.
	coreManifest := (&core.Extension{}).Manifest()

	sets := []platform.MigrationSet{}
	sets = append(sets, coreManifest.Migrations...)

	// Add extension migration sets here (reports, etc.)
	// reportsManifest := reports.New(nil).Manifest()
	// sets = append(sets, reportsManifest.Migrations...)

	runner := migration.NewRunner(pool, sets)
	if err := runner.Run(ctx); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("All migrations applied successfully.")
}
```

Note: `core.Extension{}` requires a Bundle, but we only need the Manifest (which doesn't depend on the Bundle). Refactor `core.Extension` so `Manifest()` doesn't require a populated Bundle — it should work with a zero-value Bundle:

```go
// In core.go, ensure Manifest() doesn't access e.bundle:
func (e *Extension) Manifest() platform.Manifest {
	// ... returns a static manifest, no bundle access
}
```

This is already the case in the Task 9 implementation — `Manifest()` returns a literal.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/migrate/`
Expected: Compiles without error.

- [ ] **Step 3: Commit**

```bash
git add cmd/migrate/main.go
git commit -m "refactor: migrate binary uses new versioned per-extension runner"
```

---

## Task 15: Add Makefile target for integration tests

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add the integration test target**

Append to the Makefile (after the existing `test` target):

```makefile
test-integration: ## Run integration tests (requires Docker/Podman for Testcontainers)
	go test ./... -timeout 300s -count=1 -run "DB|EndToEnd|Isolat"
```

And update `.PHONY` to include `test-integration`.

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "build: add test-integration target for Testcontainers DB tests"
```

---

## Task 16: Final verification — full build, all tests, lint

- [ ] **Step 1: Run go mod tidy**

Run: `go mod tidy`
Expected: No errors.

- [ ] **Step 2: Build everything**

Run: `go build ./...`
Expected: Compiles without error.

- [ ] **Step 3: Run all non-DB tests**

Run: `go test ./... -short -timeout 120s`
Expected: All tests PASS.

- [ ] **Step 4: Run linters**

Run: `make check`
Expected: All checks pass.

- [ ] **Step 5: Run security scanner**

Run: `make security`
Expected: No findings.

- [ ] **Step 6: Verify the boundary — reports extension imports no internals**

Run: `grep -r "internal/" extensions/reports/`
Expected: No results — the reports extension imports only `platform/` and stdlib.

- [ ] **Step 7: Final commit**

```bash
git add -A
git commit -m "chore: final verification — build, test, lint all pass"
```

---

## Self-Review Notes

### Spec coverage check

| Spec section | Implementing task(s) |
|---|---|
| §1: platform/ package + contract types | Task 1 |
| §1: capability interfaces | Task 1 (interfaces) + Task 6 (adapters) |
| §2: RouteRegistry registration | Task 2 |
| §2: Validation rules 1-6 | Task 2 (rules 1-4, 6; rule 5 is structural) |
| §2: Build to Chi + mode filtering | Task 2 |
| §2: Startup route dump | Task 4 (logRouteTable) |
| §2: Queryable registry | Task 2 (All, ByOwner, Find) |
| §2: Rich conflict diagnostics | Task 2 (validateRoutes collects all, names owners) |
| §3: platform.NewHandler assembly | Task 4 |
| §3: Core extension + Bundle | Task 9 |
| §3: main.go simplification | Task 11 |
| §3: wire.go updates | Task 10 |
| §4: Per-extension migration runner | Task 7 |
| §4: Isolated history tables | Task 7 |
| §4: Move existing migrations + idempotency | Task 8 |
| §4: cmd/migrate uses new runner | Task 14 |
| §5: Reports extension | Task 12 |
| §6: Tier 1 contract tests | Tasks 5, 12 |
| §6: Tier 2 in-memory HTTP tests | Tasks 4, 5, 12 |
| §6: Tier 3 migration tests (Testcontainers) | Task 7 |
| §6: Tier 4 real server test | (deferred — cookie test is minimal and can be added in a follow-up) |
| Plugin system deletion | Task 13 |

### Corrections from design spec

1. **Migrations are NOT idempotent** — the spec assumed `CREATE TABLE IF NOT EXISTS`. The actual SQL uses plain `CREATE TABLE`. Task 8 makes them idempotent before moving them.
2. **MessageService lacks CountMessages** — the spec's capability interface needs a passthrough. Task 6 adds it.
3. **`/dev/routes` dashboard page** — mentioned in the spec but deferred to a follow-up to keep this plan focused. The route table is logged at startup (Task 4) which provides the core observability.
4. **Metric label for route owner** — mentioned in the spec but deferred. The registry's `Owner` field makes this a small follow-up.
