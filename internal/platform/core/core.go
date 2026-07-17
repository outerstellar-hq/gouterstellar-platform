package core

import (
	"embed"
	"net/http"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

// Migrations embeds the SQL migration files shipped by the core extension.
// Files live under internal/platform/core/migrations/*.sql.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// Extension is the core platform extension. It owns the infrastructure
// routes (health, metrics, static assets, OpenAPI spec) directly and
// delegates every handler-owned route group to the handlers themselves via
// the RouteContributor interface.
type Extension struct {
	contributors []extplatform.RouteContributor
	liveness     http.Handler
	readiness    http.Handler
	robots       http.Handler
	sitemap      http.Handler
	diagnostics  http.Handler
	metrics      http.Handler
	static       http.Handler
	openapi      http.HandlerFunc
}

// NewExtension creates an empty core extension. Use the Set* and
// AddContributors methods to populate it before Contribute is called.
func NewExtension() *Extension {
	return &Extension{}
}

// SetOperations registers the process and deployment handlers owned by the
// server root because readiness depends on the live connection pool and the
// sitemap depends on runtime configuration.
func (e *Extension) SetOperations(liveness, readiness, robots, sitemap http.Handler) {
	e.liveness = liveness
	e.readiness = readiness
	e.robots = robots
	e.sitemap = sitemap
}

// SetDiagnostics registers the localhost-only route catalog handler.
func (e *Extension) SetDiagnostics(h http.Handler) { e.diagnostics = h }

// SetMetrics registers the Prometheus metrics handler.
func (e *Extension) SetMetrics(h http.Handler) { e.metrics = h }

// SetStatic registers the static-asset file server.
func (e *Extension) SetStatic(h http.Handler) { e.static = h }

// SetOpenAPI registers the OpenAPI spec handler.
func (e *Extension) SetOpenAPI(h http.HandlerFunc) { e.openapi = h }

// AddContributors appends route contributors (typically handlers) whose
// ContributeRoutes methods register the handler-owned route groups.
func (e *Extension) AddContributors(cs ...extplatform.RouteContributor) {
	e.contributors = append(e.contributors, cs...)
}

// Manifest declares the core extension's identity, mode, route ownership, and
// migrations. The core extension owns the entire platform surface
// (UI, API, admin, assets) and runs in FullPlatform mode.
func (e *Extension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "platform-core",
		Label: "Platform Core",
		Mode:  extplatform.FullPlatform,
		Ownership: extplatform.RouteOwnership{
			UI: []string{
				"/", "/auth", "/contacts", "/messages", "/search", "/settings",
				"/notifications", "/components", "/ws",
			},
			API:    []string{"/api", "/openapi.json"},
			Admin:  []string{"/admin", "/metrics"},
			Assets: []string{"/static"},
		},
		Migrations: []extplatform.MigrationSet{
			{
				ExtensionID: "platform-core",
				FS:          Migrations,
				Directory:   "migrations",
				Table:       "schema_migrations_core",
			},
		},
	}
}

// Contribute registers the core extension's routes through the platform's
// route registry. The Extension owns the infrastructure routes (health,
// metrics, static assets, OpenAPI spec) directly; every handler-owned route
// group is delegated to the handler via its RouteContributor implementation.
func (e *Extension) Contribute(ctx *extplatform.ContributionContext) error {
	// --- Infrastructure routes (owned by the Extension, not handlers) ---
	// Health probes are unauthenticated but accept only localhost Host headers.
	ctx.Routes.Public(http.MethodGet, "/health/live", "Liveness check", e.liveness)
	ctx.Routes.Public(http.MethodGet, "/health/ready", "Readiness check", e.readiness)
	ctx.Routes.Public(http.MethodGet, "/health", "Readiness check alias", e.readiness)
	ctx.Routes.Public(http.MethodGet, "/robots.txt", "Robots.txt", e.robots)
	ctx.Routes.Public(http.MethodGet, "/sitemap.xml", "Sitemap", e.sitemap)
	ctx.Routes.Public(http.MethodGet, "/debug/routes", "Local route diagnostics", e.diagnostics)
	ctx.Routes.Admin(http.MethodGet, "/metrics", "Prometheus metrics", e.metrics)
	ctx.Routes.API(http.MethodGet, "/openapi.json", "OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.API(http.MethodGet, "/api/openapi.json", "Public API OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.API(http.MethodGet, "/api/v1/sync/openapi.json", "Sync API OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/api-openapi.json", "Admin API OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.Public(http.MethodGet, "/ui/openapi.json", "Public UI OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.Protected(http.MethodGet, "/ui-protected/openapi.json", "Protected UI OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.Public(http.MethodGet, "/components/openapi.json", "Public components OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.Protected(http.MethodGet, "/components-protected/openapi.json", "Protected components OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.Admin(http.MethodGet, "/admin/openapi.json", "Admin OpenAPI spec", http.HandlerFunc(e.openapi))
	ctx.Routes.Assets("/static/*", e.static)

	// --- Handler-owned routes ---
	// Each contributor registers its own route group(s) via ctx.Routes.
	for _, c := range e.contributors {
		if err := c.ContributeRoutes(ctx); err != nil {
			return err
		}
	}

	// --- Navigation ---
	ctx.Navigation.Add("Home", "/", "house")
	ctx.Navigation.Add("Trash", "/messages/trash", "trash")
	ctx.Navigation.Add("Contacts", "/contacts", "users")
	ctx.Navigation.Add("Search", "/search", "search")
	ctx.Navigation.Add("Settings", "/settings", "gear")
	ctx.Navigation.Add("Notifications", "/notifications", "bell")

	return nil
}
