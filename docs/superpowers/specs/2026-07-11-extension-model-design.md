# Extension Model Design

**Date:** 2026-07-11
**Status:** Approved (pending spec review)
**Reconciles:** `outerstellar-go-platform-agent-brief.md` (the upstream brief) against the existing `gouterstellar-platform` codebase.

## Context and Problem

The upstream brief defines a compile-time extension model as the architectural centerpiece of the Go platform: extensions declare manifests, contribute routes through a validated registry, own route prefixes, and fail fast on conflicts. The existing codebase implements much of the brief's *infrastructure* (Chi, pgx, sqlc, security middleware, single binary) but **does not implement the extension model at all**. Routes are wired directly in `cmd/server/main.go` via each handler's `RegisterRoutes(chi.Router)`, there is no route registry, no ownership validation, no conflict detection, and no platform modes. A separate `pkg/plugin/` system exists but is inert (instantiated in `wire.go:181`, never discovered, never registered, never shut down).

This design retrofits the brief's extension model into the existing codebase. The goal is to satisfy the brief's acceptance criteria without rewriting the working handler/service/persistence layers.

## Decisions (from brainstorming)

| Decision | Choice |
|---|---|
| Plugin system coexistence | **Replace** `pkg/plugin/` entirely with the brief's Extension model |
| Extension boundary | **Compile-time, in-tree** extensions in the same binary |
| Public API surface | **Strict public boundary** — a new `platform/` package that extensions import; no `internal/` imports |
| Existing handlers | **Core extension** — all 20 existing handlers become one `platform-core` extension contributing through the registry |
| Ownership enforcement | **Full** — all six validation rules enforced as hard startup failures from day one |
| Platform modes | **All three** — `FullPlatform`, `ExtensionHost`, `Headless` |
| Migrations | **Per-extension migration sets** with isolated history tables, replacing the naive re-apply-everything runner |
| Sample extension | **New `reports` extension** built from scratch as the reference |
| DB test strategy | **Testcontainers-Go** for migration tests against real PostgreSQL |

## Approach

**Registry-as-bus, core stays monolithic.** A new `platform/` public package defines the extension contract and route registry. The `wire.Wire` composition root gains a new assembly phase that constructs extensions, runs contribution, validates, and builds the Chi router from the registry. The existing 20 handlers become a single `core` extension. The naive migration runner is replaced by a versioned per-extension runner. A new `reports` extension proves the model.

## Section 1: The `platform/` package and extension contract

New top-level package: `github.com/outerstellar-hq/gouterstellar-platform/platform`

The package imports **nothing from `internal/`**. It depends only on the standard library and Chi (for `http.Handler` and route patterns flowing through it).

### Extension interface and manifest

```go
// platform/extension.go
package platform

type Extension interface {
    Manifest() Manifest
    Contribute(ctx *ContributionContext) error
}

type Manifest struct {
    ID         string
    Label      string
    Mode       PlatformMode
    Ownership  RouteOwnership
    Migrations []MigrationSet
}

type PlatformMode string
const (
    FullPlatform  PlatformMode = "full"
    ExtensionHost PlatformMode = "extension-host"
    Headless      PlatformMode = "headless"
)

type RouteOwnership struct {
    UI     []string
    API    []string
    Admin  []string
    Assets []string
}

type MigrationSet struct {
    ExtensionID string
    FS          fs.FS
    Directory   string
    Table       string  // isolated history table per extension
}
```

### Contribution context

The capability surface — what extensions are allowed to touch. Typed registries rather than raw services, so extensions never depend on `internal/`:

```go
type ContributionContext struct {
    Routes        *RouteRegistry
    Navigation    *NavigationRegistry
    PlatformPages *PlatformPageRegistry
    Assets        *AssetRegistry
    Admin         *AdminRegistry
}
```

Each registry is a small, focused type with methods like `Routes.Public(method, pattern, desc, handler)`, `Navigation.Add(label, url, icon)`. They collect metadata during contribution; the platform validates and builds the Chi router from them afterward.

**Ownership binding:** the `ContributionContext` is constructed *per extension*. `ctx.Routes` is wired with the owning extension's ID so every `Public(...)`/`API(...)` call stamps that owner onto the registration. An extension cannot register routes under another extension's ID.

### Service capability interfaces

Extensions that need data access get small interfaces defined in `platform/`. The wire root satisfies these by adapting internal services:

```go
// platform/capabilities.go
type MessageCounter interface {
    CountMessages(ctx context.Context) (int64, error)
}
```

Extensions depend on the interface, not the concrete `*service.MessageService`. The wire root builds a `ServiceBag` of these capability adapters and passes them to extensions via constructors.

## Section 2: Route registry, validation, and observability

The registry collects route registrations during contribution. No Chi router exists yet. After all extensions contribute, a single `Validate` pass checks every rule, then `Build(chi.Router)` mounts the survivors.

### Registration

```go
// platform/route_registry.go
type RouteGroup string
const (
    GroupPublicUI    RouteGroup = "public-ui"
    GroupProtectedUI RouteGroup = "protected-ui"
    GroupAPI         RouteGroup = "api"
    GroupAdmin       RouteGroup = "admin"
    GroupAssets      RouteGroup = "assets"
)

type RouteRegistration struct {
    Owner       string         // extension ID
    Method      string
    Pattern     string
    Group       RouteGroup
    Description string
    Handler     http.Handler
}

type RouteRegistry struct {
    routes []RouteRegistration
}

func (r *RouteRegistry) Public(method, pattern, desc string, h http.Handler)
func (r *RouteRegistry) Protected(method, pattern, desc string, h http.Handler)
func (r *RouteRegistry) API(method, pattern, desc string, h http.Handler)
func (r *RouteRegistry) Admin(method, pattern, desc string, h http.Handler)
func (r *RouteRegistry) Assets(pattern string, h http.Handler)
```

### Validation rules (all enforced as hard startup failures)

| # | Rule | Error message shape |
|---|------|---------------------|
| 1 | Path must be absolute | `route path must be absolute: reports contributed "reports/home"` |
| 2 | Route inside owner's declared prefixes | `route GET /settings is outside ownership of reports (allowed: /reports, /extension/reports)` |
| 3 | No duplicate method+path across owners | `route conflict: GET /settings is owned by both platform-core and reports` |
| 4 | Headless mode rejects HTML groups | `headless mode rejects HTML route GET / owned by platform-core` |
| 5 | Extension routes don't conflict with enabled platform pages | `route GET / conflicts with platform page owned by platform-core` |

**"Enabled platform pages"** (rule 5) are the UI routes that core contributes in `FullPlatform` and `ExtensionHost` modes — `/`, `/auth`, `/settings`, etc. These are determined *after* core contributes, by reading core's registrations with `GroupProtectedUI` or `GroupPublicUI`. In `ExtensionHost` mode, core may suppress its root `/` page (configurable), freeing it for another extension to own. Rule 5 then checks that no two extensions claim the same platform page route. In `Headless` mode, platform pages aren't contributed at all, so rule 5 is moot.
| 6 | Asset paths inside declared asset ownership | `asset path /static/reports/x.css is outside asset ownership of reports` |

The validator collects **all** conflicts before failing — it does not abort on the first one. Rich conflict diagnostics carry context so you don't have to re-run:

```text
route conflict: GET /settings
  first registered by:  platform-core (manifest declared ownership: /settings, /api/v1/*)
  also registered by:   reports      (manifest declared ownership: /reports, /api/v1/reports)
  suggestion: reports should declare /settings in its RouteOwnership.UI, or remove the duplicate route
```

### Two mode concepts — clarified

There are two mode fields that must not be confused:

- **`Options.Mode`** (platform-wide) — the operating mode the *application* runs in. Set once at startup. Determines which route groups are eligible and what UI the platform mounts.
- **`Manifest.Mode`** (per-extension) — declares which mode the extension is *designed for*. An informational tag the builder uses to decide permission grants. An extension declaring `ExtensionHost` is signaling that it expects to own root UI routes; the builder grants that privilege only when the platform is also running in `ExtensionHost` mode.

An extension whose `Manifest.Mode` doesn't match the running `Options.Mode` isn't rejected outright — its API and non-root routes still contribute. But the builder won't grant the root-UI ownership privilege unless both modes agree on `ExtensionHost`.

### Mode behavior in the builder

- **FullPlatform:** the core extension contributes default platform pages (`/`, `/auth`, `/settings`, etc.). All groups mount. Root UI routes (`/`) are owned by core only.
- **ExtensionHost:** core still contributes auth + API (the platform backbone), but root UI routes (`/`) can be owned by a non-core extension. The builder allows one non-core owner for a root UI prefix only when both `Options.Mode` and the extension's `Manifest.Mode` are `ExtensionHost`. Core may optionally suppress its own root UI contribution in this mode (configurable).
- **Headless:** the builder drops every registration whose `Group` is `GroupPublicUI`, `GroupProtectedUI`, or `GroupAdmin`. Only `GroupAPI` and `GroupAssets` survive. Rule 4 enforces this at validate time.

### Build phase

`registry.Build(r chi.Router, mode PlatformMode)` mounts validated routes onto the Chi router, grouped by `RouteGroup`. The API group gets `BearerAuth` middleware applied; the admin group gets the admin-only permission check; HTML groups get the session/CSRF chain that's already on the global middleware. This centralizes what is currently scattered across `main.go` lines 87-113.

### Registry observability

**1. Startup route dump to logs.** After validation succeeds and before the server listens, log the complete routing table grouped by owner:

```
route registry validated: 47 routes across 2 extensions
  platform-core (full):
    GET    /                      [protected-ui]
    POST   /api/v1/auth/login     [api]
    GET    /admin/users           [admin]
    ...
  reports (extension-host):
    GET    /reports               [protected-ui]
    GET    /api/v1/reports/summary [api]
```

**2. Queryable registry API** for runtime introspection:

```go
func (r *RouteRegistry) All() []RouteRegistration
func (r *RouteRegistry) ByOwner(id string) []RouteRegistration
func (r *RouteRegistry) Find(method, pattern string) (RouteRegistration, bool)
```

This feeds into the existing `DevDashboardHandler` — a `/dev/routes` page rendering the table with owner, group, and description columns. Gated behind `cfg.DevDashboardEnabled`.

**3. Rich conflict diagnostics** (shown above) — all conflicts at once, with context and suggestions.

**4. Metric label.** The brief asks for `route owner` as a Prometheus metric label. The registry's `Owner` field lets the metrics middleware tag the request-duration histogram with the matched registration's owner from request context.

## Section 3: Platform assembly and the core extension

### New assembly flow

Today: `wire.Wire` → flat `App` struct → `main.go` builds Chi router by calling `handler.RegisterRoutes` directly.

After: `wire.Wire` builds services/handlers as today, then a new assembly step (`platform.NewHandler`) constructs extensions, runs contribution, validates, and builds the router. `main.go` no longer touches Chi directly.

```
wire.Wire(cfg, pool, templateFS)
  ├── builds repos, services, handlers (unchanged)
  ├── constructs capability adapters (NEW — wraps services as platform.MessageCounter, etc.)
  └── returns *App with handler dependencies ready

platform.NewHandler(options) → http.Handler   (NEW — the assembly step)
  ├── 1. construct RouteRegistry, NavigationRegistry, etc.
  ├── 2. for each extension: build per-extension ContributionContext (routes stamped with owner)
  ├── 3. call extension.Contribute(ctx) for each — routes/nav/assets collected
  ├── 4. registry.Validate(mode, enabledPlatformPages) — all 6 rules, all conflicts at once
  ├── 5. log the full route table (Section 2 observability)
  ├── 6. registry.Build(chiRouter, mode) — mounts validated routes, applies group middleware
  └── returns the chiRouter as http.Handler
```

Both production and tests call `platform.NewHandler(options)` — satisfying the brief's *"make test construction use the same assembly path as production."*

### Options type

```go
type Options struct {
    Mode             PlatformMode
    Extensions       []Extension
    Services         ServiceBag
    MiddlewareChain  []func(http.Handler) http.Handler
    Config           ConfigOptions
}

type ServiceBag struct {
    MessageCounter   MessageCounter    // adapts *service.MessageService
    ContactCounter   ContactCounter    // adapts *service.ContactService
    UserStore        UserStore         // adapts *service.SecurityService
    SessionAuth      SessionAuth       // adapts the session realm
    // … grows as extensions need more; each is a small interface in platform/
}
```

### Core extension

New package `internal/platform/core/` contains the `CoreExtension`. It wraps the existing handlers and contributes all their routes through the registry:

```go
func (e *CoreExtension) Manifest() platform.Manifest {
    return platform.Manifest{
        ID:   "platform-core",
        Label: "Platform Core",
        Mode:  platform.FullPlatform,
        Ownership: platform.RouteOwnership{
            UI:     []string{"/", "/auth", "/contacts", "/search", "/settings", "/notifications", "/components", "/ws"},
            API:    []string{"/api/v1"},
            Admin:  []string{"/admin"},
            Assets: []string{"/static"},
        },
        Migrations: []platform.MigrationSet{
            {ExtensionID: "platform-core", FS: coreMigrations, Directory: "migrations", Table: "schema_migrations_core"},
        },
    }
}
```

### Adapter layer

Each existing handler keeps its struct and service dependencies untouched. What changes is that `RegisterRoutes(chi.Router)` is no longer called directly by `main.go`. Instead, the `CoreExtension.Contribute` method maps the current `r.Get(pattern, h.Show)` calls into `ctx.Routes.Protected(method, pattern, desc, h.Show)` calls. The mapping from the current `main.go` route blocks (lines 87-113) to registry calls is mechanical:

- Lines 87-94 (`r.Route("/", ...)` with `BearerAuth`) → `GroupAPI` registrations. `BearerAuth` applied by the builder to the whole group.
- Lines 96-104 (HTML handlers) → `GroupProtectedUI` (auth-required pages) or `GroupPublicUI` (`/auth` login/register).
- Lines 106-111 (admin) → `GroupAdmin`.

### Deletions

- `pkg/plugin/` entirely (`plugin.go`, `manager.go`, `manager_test.go`).
- `App.PluginManager` field and construction (`wire.go:181`).
- Direct route-registration block in `main.go` (lines 48-118) — replaced by a single `http.Handler` from `platform.NewHandler`.

### Resulting main.go

After the change, `main.go` shrinks to: load config → connect pool → `wire.Wire` → `platform.NewHandler(options)` → `srv.ListenAndServe(handler)`. The global middleware chain (request ID, logging, CSRF, session, etc.) is passed into `Options.MiddlewareChain`. Background goroutines (outbox processor, session cleanup) stay in `main.go` — they're not route concerns.

## Section 4: Per-extension migrations

Replaces the naive migration runner (`cmd/migrate/main.go`) that re-applies every SQL file every run with no version tracking.

### Problem with the current runner

`cmd/migrate/main.go` reads `migrations/*.sql`, sorts lexically, and `pool.Exec`s each file's entire content on every run. No schema migrations table. SQL files must be idempotent. No way to know if a migration ran. No rollback path. Extensions can't own schema.

### New model

Per-extension `MigrationSet` declares an isolated history table per extension. Execution order is deterministic: platform-core first, then extensions in lexicographic order by `ExtensionID`.

### Migration runner

New `platform/migration` package contains the runner (~150 lines):

```go
type Runner struct {
    pool   *pgxpool.Pool
    sets   []platform.MigrationSet
}

func (r *Runner) Run(ctx context.Context) error {
    for _, set := range r.sortedSets() {
        if err := r.ensureHistoryTable(ctx, set); err != nil { return err }
        applied, err := r.appliedVersions(ctx, set)
        if err != nil { return err }
        pending, err := r.pendingMigrations(set, applied)
        if err != nil { return err }
        for _, m := range pending {
            if err := r.applyOne(ctx, set, m); err != nil { return err }
        }
    }
    return nil
}
```

`applyOne` wraps each migration in a transaction and records the version in the extension's history table.

### Version tracking table (one per extension, created lazily)

```sql
CREATE TABLE IF NOT EXISTS schema_migrations_core (
    version    BIGINT PRIMARY KEY,
    filename   TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Version convention

SQL files named `V001__description.sql` (zero-padded for clean lexical sort, extending the existing `V1__`, `V2__` convention). Version is the integer parsed from the `V<NNN>` prefix.

### Migration of existing SQL files

The four existing files in `migrations/` move into an embedded FS owned by the core extension:

```
internal/platform/core/
├── migrations/
│   ├── V001__initial_schema.sql          (was V1__initial_schema.sql)
│   ├── V002__user_profile_enhancements.sql
│   ├── V003__sessions_table.sql
│   └── V004__user_preferences.sql
├── migrations.go                         // go:embed
```

**First-run transition:** the existing SQL files use `CREATE TABLE IF NOT EXISTS` — they're idempotent. On first run under the new runner: `ensureHistoryTable` creates `schema_migrations_core`, `appliedVersions` returns empty, all four migrations re-execute (safe no-ops for existing tables), all four recorded as version 1-4. From the second run onward, they're skipped. Future core migrations can be non-idempotent because the version table guarantees single application.

This is called out explicitly so it's not a surprise: **the first run under the new runner re-executes the four existing migrations.** Idempotency makes this safe; it's a one-time cost.

### cmd/migrate binary

It shrinks. Instead of the hand-rolled apply-everything logic, it constructs the same extension list that the server uses and calls `migration.Runner.Run`. `buildMigrationSets` is shared between `cmd/migrate` and `cmd/server` so the two never disagree about which extensions exist.

### Observability

The runner logs each applied migration with extension ID, version, filename, and duration. On failure, the error identifies the extension, version, and filename.

## Section 5: The sample `reports` extension

Reference extension proving the model works. Demonstrates manifest, ownership, route contribution, navigation, own migration, and data access through capability interfaces — all without importing `internal/`.

### Location and structure

```
extensions/reports/
├── reports.go              // Extension type, Manifest, Contribute
├── handlers.go             // HTTP handlers (home page + JSON API)
├── migrations.go           // go:embed
└── migrations/
    └── V001__reports_tables.sql
```

Lives at top level (not inside `internal/`). Imports only `github.com/outerstellar-hq/gouterstellar-platform/platform`. If it compiles without touching `internal/`, the boundary is proven.

### Extension

```go
package reports

type Extension struct {
    messages platform.MessageCounter
}

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

func (e *Extension) Contribute(ctx *platform.ContributionContext) error {
    ctx.Routes.Protected(http.MethodGet, "/reports", "Reports home", e.home)
    ctx.Routes.API(http.MethodGet, "/api/v1/reports/summary", "Message count summary", e.summary)
    ctx.Navigation.Add("Reports", "/reports", "bar-chart")
    return nil
}
```

Manifest declares `Mode: ExtensionHost` and owns `/reports` (not `/` — that would conflict with core's home dashboard). Proves route ownership and navigation contribution without manufacturing a conflict with core.

### Migration

```sql
CREATE TABLE IF NOT EXISTS reports_snapshots (
    id           BIGSERIAL PRIMARY KEY,
    captured_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_count BIGINT NOT NULL,
    contact_count BIGINT NOT NULL
);
```

Small, self-contained, owned by the extension. Proves extensions can evolve their own schema independently.

### Handlers

Deliberately minimal HTML — no templ dependency for the PoC. The brief lists templ as a stack choice but it's not in the current repo (it uses `html/template`). The reports extension renders plain HTML to avoid introducing a new dependency this cycle. A follow-up could adopt templ across the board.

### Capability adapter

The wire root injects `platform.MessageCounter` into the extension. The adapter is in `internal/platform/`:

```go
type messageCounterAdapter struct{ svc *service.MessageService }

func (a messageCounterAdapter) CountMessages(ctx context.Context) (int64, error) {
    return a.svc.CountMessages(ctx)
}
```

This is the only place `internal/` meets `platform/` — the wire root is the seam.

### Assembly wiring

```go
handler, err := platform.NewHandler(platform.Options{
    Mode: platform.FullPlatform,
    Extensions: []platform.Extension{
        core.New(coreDeps),
        reports.New(svcBag.MessageCounter),
    },
    Services:        svcBag,
    MiddlewareChain: globalMiddleware,
    Config:          platformCfg,
})
```

### How reports proves each acceptance criterion

| Brief criterion | How reports demonstrates it |
|---|---|
| Extensions composed explicitly | Listed in `Options.Extensions` |
| Invalid route ownership fails before startup | If reports tried to register `/settings`, validation fails |
| Route conflicts fail before startup | If reports and core both claimed `/reports`, the error names both |
| Entire router testable without a port | `platform.NewTestApp(reports.New(...))` → `handler.ServeHTTP(rec, req)` |
| Sample extension requires no imports from platform internals | `extensions/reports/` imports only `platform/` + stdlib |

## Section 6: Testing strategy

Four tiers matching the brief, each testing a different concern.

### Tier 1: Extension contract tests (no platform startup)

Validate that an extension declares correctly — manifest, ownership, contributions — without building a router. Fastest tests.

```go
func TestReportsContribution(t *testing.T) {
    diagnostics, err := platform.CheckExtension(
        reports.New(stubMessageCounter{}),
        platform.TestHostContext(),
    )
    require.NoError(t, err)
    require.ElementsMatch(t,
        []string{"GET /reports", "GET /api/v1/reports/summary"},
        diagnostics.RoutePatterns(),
    )
    require.NoError(t, diagnostics.OwnershipViolations())
    require.Contains(t, diagnostics.NavigationLabels(), "Reports")
}
```

`platform.CheckExtension` constructs a throwaway `ContributionContext` with a real `RouteRegistry`, calls `Contribute`, returns `Diagnostics`. Checks: manifest well-formed, contributed routes inside declared ownership, migrations reference existing embedded directory, no internal duplicate routes.

### Tier 2: In-memory full-stack HTTP tests

Exercise the real assembly path (router construction, middleware chain, validation, build) without opening a port.

```go
func TestReportsHome(t *testing.T) {
    app, err := platform.NewTestApp(platform.TestOptions{
        Extensions: []platform.Extension{
            core.NewTest(),
            reports.New(stubMessageCounter{}),
        },
    })
    require.NoError(t, err)
    t.Cleanup(app.Close)

    req := httptest.NewRequest(http.MethodGet, "/reports", nil)
    rec := httptest.NewRecorder()
    app.Handler.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    require.Contains(t, rec.Body.String(), "Reports")
}
```

`platform.NewTestApp` builds the handler through the same `platform.NewHandler` assembly path as production, substituting lightweight stubs for database-backed services. Covers: routing, middleware ordering, authentication, validation at startup, conflict diagnostics.

**Conflict test — the brief's headline scenario:**

```go
func TestRouteConflictFailsAtStartup(t *testing.T) {
    _, err := platform.NewTestApp(platform.TestOptions{
        Extensions: []platform.Extension{
            core.NewTest(),
            conflictingExtension{},   // also registers GET /reports
        },
    })
    require.Error(t, err)
    require.Contains(t, err.Error(), "route conflict: GET /reports")
    require.Contains(t, err.Error(), "platform-core")
    require.Contains(t, err.Error(), "conflicting-extension")
}
```

### Tier 3: Migration tests (Testcontainers for Go)

**Pure-logic tests (fast, no Docker):** version parsing from filenames, pending-migration filtering, ordering (core before extensions, extensions by ID), history-table SQL well-formedness.

**End-to-end DB tests (Testcontainers-Go, slower):** runner against a fresh Postgres container, verify tables created, history recorded, re-run is a no-op, extension migration isolation (reports → `schema_migrations_reports`, core → `schema_migrations_core`), the transition case (existing idempotent migrations re-applied once, then skipped on second run).

Adding `testcontainers-go` as a dev dependency this cycle. It's a one-line `go get` and a `TestMain` that boots a Postgres container.

### Tier 4: Real server tests (minimal)

Only for things that genuinely need transport behavior. One test: cookie behavior through a real client (`httptest.NewServer` + `http.Client` with cookie jar, verify session cookie persists across requests).

### Test file inventory

| File | Tier | What it tests |
|---|---|---|
| `platform/route_registry_test.go` | 1 | Registry add, ownership validation rules 1-6, conflict detection, mode filtering |
| `platform/migration/runner_test.go` | 3 (logic) | Version parsing, pending filter, ordering |
| `platform/migration/runner_db_test.go` | 3 (db) | End-to-end against Postgres via Testcontainers |
| `extensions/reports/reports_test.go` | 1 | Reports manifest, contributions, ownership |
| `extensions/reports/reports_http_test.go` | 2 | In-memory HTTP for reports routes |
| `internal/platform/core/contribute_test.go` | 1 | Core extension contributes all expected routes |
| `platform/handler_test.go` | 2 | Full assembly, conflict detection, mode behavior |

## Acceptance criteria

This design satisfies the brief's acceptance criteria:

- [x] Application builds as one Go binary
- [x] Extensions composed explicitly (listed in `Options.Extensions`)
- [x] Invalid route ownership fails before startup (registry validation, rule 2)
- [x] Route conflicts fail before startup (registry validation, rule 3, all conflicts at once)
- [x] Entire router testable without opening a port (`platform.NewTestApp` → `ServeHTTP`)
- [x] HTML pages render (existing `html/template` renderer preserved; reports extension renders minimal HTML)
- [x] JSON APIs work (existing API handlers register through `GroupAPI`)
- [x] Authentication works (existing session/apikey/jwt realms preserved; builder applies to API group)
- [x] Permission middleware works (admin group gets permission check)
- [x] Platform and extension migrations run correctly (per-extension runner with isolated history)
- [x] A PostgreSQL integration test passes (Testcontainers)
- [x] Observability records route owner and request metrics (metric label + startup dump)
- [x] Sample extension requires no imports from platform internals (`extensions/reports/` imports only `platform/`)

## Out of scope for this cycle

- Decomposing the core extension into multiple feature-area extensions (natural follow-up)
- Adopting templ (reports extension uses plain HTML; templ adoption is a separate cycle)
- Adopting Huma / OpenAPI generation (separate cycle)
- Adopting HTMX (separate cycle)
- Adopting Koanf (config stays on Viper)
- Adopting Goose specifically (custom thin runner implemented instead, per brief's *"create an abstraction around the migration runner"* guidance)
- Out-of-tree compiled extensions (in-tree only for this PoC)
- Dynamic runtime-loaded extensions (explicitly deferred by the brief)
