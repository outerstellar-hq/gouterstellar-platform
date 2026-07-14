package core

import (
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

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
	ctx.Navigation.Add("Contacts", "/contacts", "users")
	ctx.Navigation.Add("Search", "/search", "search")
	ctx.Navigation.Add("Settings", "/settings", "gear")
	ctx.Navigation.Add("Notifications", "/notifications", "bell")

	return nil
}
