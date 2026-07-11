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
// It: validates manifests -> runs contribution -> validates routes -> builds router.
// Returns an error if any validation fails; the error includes ALL failures.
func NewHandler(opts Options) (http.Handler, error) {
	// 1. Validate all manifests first.
	ownershipMap := make(map[string]RouteOwnership)
	for _, ext := range opts.Extensions {
		m := ext.Manifest()
		if err := m.Validate(); err != nil {
			return nil, fmt.Errorf("extension %s: %w", m.ID, err)
		}
		ownershipMap[m.ID] = m.Ownership
	}

	// 2. Run contribution for each extension.
	var allRoutes []RouteRegistration
	var allNav []NavigationItem
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

func formatValidationErrors(errs []error) error {
	msg := fmt.Sprintf("%d route validation error(s):\n", len(errs))
	for _, e := range errs {
		msg += fmt.Sprintf("  - %s\n", e.Error())
	}
	return fmt.Errorf("%s", msg)
}

func logRouteTable(routes []RouteRegistration, nav []NavigationItem) {
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
