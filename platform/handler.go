package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

// Options configures the platform handler assembly.
type Options struct {
	Mode            PlatformMode
	Extensions      []Extension
	Services        ServiceBag
	MiddlewareChain []func(http.Handler) http.Handler
	// StaticDir optionally overrides packaged platform and extension assets.
	StaticDir string
	// AssetMiddleware applies host caching policy to every asset route.
	AssetMiddleware func(http.Handler) http.Handler
	// GroupMiddleware applies middleware to specific route groups after
	// the global MiddlewareChain. Keys are RouteGroup values
	// (GroupAPI, GroupAdmin, etc.). The middleware is applied via Chi
	// route groups, so it only affects routes in that group.
	GroupMiddleware map[RouteGroup][]func(http.Handler) http.Handler
	// NotFoundHandler renders unmatched requests after extension routes mount.
	// When nil, Chi's default plain-text 404 response is retained.
	NotFoundHandler http.Handler
	// Catalog receives the validated, mounted extension and route inventory.
	// It may be shared with diagnostic handlers created before assembly.
	Catalog *Catalog
}

// NewHandler assembles the complete web application as an http.Handler.
// It: validates manifests -> runs contribution -> validates routes -> builds router.
// Returns an error if any validation fails; the error includes ALL failures.
func NewHandler(opts Options) (http.Handler, error) {
	if opts.Mode == ExtensionHost {
		slog.Info("running in ExtensionHost mode — extensions may own root UI routes")
	} else if opts.Mode == Headless {
		slog.Info("running in Headless mode — HTML routes suppressed")
	} else {
		slog.Info("running in FullPlatform mode")
	}

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
	var allBanners []bannerRegistration
	var allReadiness []ReadinessStatus
	for _, ext := range opts.Extensions {
		ctx := newContributionContext(ext.Manifest().ID, opts.Services, assetHostOptions{
			staticDir:       opts.StaticDir,
			assetMiddleware: opts.AssetMiddleware,
		})
		if err := ext.Contribute(ctx); err != nil {
			return nil, fmt.Errorf("extension %s contribute: %w", ext.Manifest().ID, err)
		}
		allRoutes = append(allRoutes, ctx.Routes.All()...)
		allNav = append(allNav, ctx.Navigation.Items()...)
		allBanners = append(allBanners, ctx.Banners.all()...)
		allReadiness = append(allReadiness, ctx.Readiness.All()...)
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

	// Inject extension-contributed nav items into each request's context so
	// the renderer can read them at render time without middleware plumbing
	// through the handler layer. The items are static after assembly, so we
	// convert once here and reuse the slice for every request.
	navVM := convertNavItems(allNav)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestContext := RequestContextFrom(req)
			if csrfToken := web.CSRFTokenFromRequest(req); csrfToken != "" {
				requestContext.CSRFToken = csrfToken
			}
			if requestID := web.RequestIDFromContext(req.Context()); requestID != "" {
				requestContext.RequestID = requestID
			}
			if user := web.UserFromRequest(req); user != nil {
				requestContext.User = &RequestUser{
					ID:       user.ID.String(),
					Username: user.Username,
					Role:     string(user.Role),
					IsAdmin:  user.Role == model.RoleAdmin,
				}
			}
			req = withRequestContext(req, requestContext)
			if requestContext.User != nil && len(allBanners) > 0 {
				user := *requestContext.User
				req = web.WithBannerLoader(req, func(ctx context.Context) ([]viewmodel.Banner, error) {
					return resolveBanners(ctx, user, allBanners)
				})
			}
			next.ServeHTTP(w, web.WithNavItems(req, navVM))
		})
	})

	mounted := buildRoutes(r, allRoutes, opts.Mode, ownershipMap, opts.GroupMiddleware)
	if opts.NotFoundHandler != nil {
		r.NotFound(opts.NotFoundHandler.ServeHTTP)
	}
	opts.Catalog.replace(opts.Extensions, mounted, allReadiness)

	// 5. Log the route table (observability).
	logRouteTable(mounted, allNav)

	return r, nil
}

func resolveBanners(ctx context.Context, user RequestUser, registrations []bannerRegistration) ([]viewmodel.Banner, error) {
	var banners []Banner
	for _, registration := range registrations {
		provided, err := registration.provider.Banners(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("extension %s banners: %w", registration.owner, err)
		}
		for _, banner := range provided {
			if err := validateBanner(registration.owner, banner); err != nil {
				return nil, err
			}
			banners = append(banners, banner)
		}
	}

	sort.SliceStable(banners, func(i, j int) bool {
		return bannerPriority(banners[i].Severity) < bannerPriority(banners[j].Severity)
	})

	resolved := make([]viewmodel.Banner, len(banners))
	for i, banner := range banners {
		resolved[i] = viewmodel.Banner{
			ID:          banner.ID,
			Title:       banner.Title,
			Body:        banner.Body,
			Severity:    string(banner.Severity),
			Dismissible: banner.Dismissible && banner.DismissURL != "",
			DismissURL:  banner.DismissURL,
		}
	}
	return resolved, nil
}

func validateBanner(owner string, banner Banner) error {
	if strings.TrimSpace(banner.ID) == "" {
		return fmt.Errorf("extension %s banner ID must not be empty", owner)
	}
	if strings.TrimSpace(banner.Title) == "" {
		return fmt.Errorf("extension %s banner %s title must not be empty", owner, banner.ID)
	}
	if bannerPriority(banner.Severity) < 0 {
		return fmt.Errorf("extension %s banner %s has invalid severity %q", owner, banner.ID, banner.Severity)
	}
	if banner.DismissURL != "" && !sameOriginPath(banner.DismissURL) {
		return fmt.Errorf("extension %s banner %s dismiss URL must be a same-origin absolute path", owner, banner.ID)
	}
	return nil
}

func bannerPriority(severity BannerSeverity) int {
	switch severity {
	case BannerCritical:
		return 0
	case BannerWarning:
		return 1
	case BannerInfo:
		return 2
	case BannerMaintenance:
		return 3
	default:
		return -1
	}
}

func sameOriginPath(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") &&
		!strings.HasPrefix(raw, "//") && !strings.Contains(raw, `\`)
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

// convertNavItems maps the platform's contribution-time NavigationItem into the
// renderer's viewmodel.NavItem. Active state is resolved per-request in the
// renderer (it depends on the current path), so it is left false here.
func convertNavItems(items []NavigationItem) []viewmodel.NavItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]viewmodel.NavItem, len(items))
	for i, item := range items {
		out[i] = viewmodel.NavItem{
			Label: item.Label,
			URL:   item.URL,
			Icon:  item.Icon,
		}
	}
	return out
}

// TestOptions configures a test application.
type TestOptions struct {
	Mode            PlatformMode
	Extensions      []Extension
	Services        ServiceBag
	MiddlewareChain []func(http.Handler) http.Handler
	GroupMiddleware map[RouteGroup][]func(http.Handler) http.Handler
}

// TestApp is the result of NewTestApp. It exposes the assembled handler so
// in-memory HTTP tests can drive it with httptest without starting real
// servers or databases.
type TestApp struct {
	Handler http.Handler
}

// NewTestApp builds the handler through the same NewHandler assembly path as
// production. This means the same manifest validation, contribution, route
// validation, and router build steps run in tests as in the live wire root.
func NewTestApp(opts TestOptions) (*TestApp, error) {
	handler, err := NewHandler(Options{
		Mode:            opts.Mode,
		Extensions:      opts.Extensions,
		Services:        opts.Services,
		MiddlewareChain: opts.MiddlewareChain,
		GroupMiddleware: opts.GroupMiddleware,
	})
	if err != nil {
		return nil, err
	}
	return &TestApp{Handler: handler}, nil
}
