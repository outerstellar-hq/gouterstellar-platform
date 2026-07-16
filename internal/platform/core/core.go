package core

import (
	"embed"
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
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
	health       http.HandlerFunc
	metrics      http.Handler
	static       http.Handler
	openapi      http.HandlerFunc
}

// NewExtension creates an empty core extension. Use the Set* and
// AddContributors methods to populate it before Contribute is called.
func NewExtension() *Extension {
	return &Extension{}
}

// SetHealth registers the health-check handler (owned by the server root
// because it needs the live connection pool).
func (e *Extension) SetHealth(h http.HandlerFunc) { e.health = h }

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
			API:    []string{"/api/v1", "/metrics", "/openapi.json"},
			Admin:  []string{"/admin", "/dev"},
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
	// /health is public and unauthenticated so orchestrators can probe it.
	ctx.Routes.Public(http.MethodGet, "/health", "Health check", http.HandlerFunc(e.health))
	ctx.Routes.API(http.MethodGet, "/metrics", "Prometheus metrics", e.metrics)
	ctx.Routes.API(http.MethodGet, "/openapi.json", "OpenAPI spec", http.HandlerFunc(e.openapi))
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
	ctx.Navigation.Add("Messages", "/messages", "message-square")
	ctx.Navigation.Add("Trash", "/messages/trash", "trash")
	ctx.Navigation.Add("Contacts", "/contacts", "users")
	ctx.Navigation.Add("Search", "/search", "search")
	ctx.Navigation.Add("Settings", "/settings", "gear")
	ctx.Navigation.Add("Notifications", "/notifications", "bell")

	return nil
}
