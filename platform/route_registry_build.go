package platform

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// buildRoutes mounts the given registrations onto a Chi router, skipping
// HTML groups in headless mode. Routes are grouped by RouteGroup so that
// group-specific middleware (e.g. BearerAuth for API, permission checks
// for Admin) can be applied via groupMiddleware.
// Registration is assumed to have already been validated.
func buildRoutes(
	r chi.Router,
	routes []RouteRegistration,
	mode PlatformMode,
	ownership map[string]RouteOwnership,
	groupMiddleware map[RouteGroup][]func(http.Handler) http.Handler,
) []RouteRegistration {
	var mounted []RouteRegistration

	// Build a set of groups that have middleware, so we can use Chi Group.
	// Chi's r.Group creates an inline sub-router that inherits the parent's
	// middleware and adds its own.
	type groupContext struct {
		router chi.Router
	}
	groups := make(map[RouteGroup]*groupContext)

	for _, reg := range routes {
		if mode == Headless && isHTMLGroup(reg.Group) {
			continue
		}

		// Lazily create group sub-routers with their middleware.
		target := r
		if mws, ok := groupMiddleware[reg.Group]; ok && len(mws) > 0 {
			if _, exists := groups[reg.Group]; !exists {
				gr := r.Group(func(gr chi.Router) {
					for _, mw := range mws {
						gr.Use(mw)
					}
				})
				groups[reg.Group] = &groupContext{router: gr}
			}
			target = groups[reg.Group].router
		}

		mountRoute(target, reg)
		mounted = append(mounted, reg)
	}

	return mounted
}

// mountRoute registers a single route on the given router.
func mountRoute(r chi.Router, reg RouteRegistration) {
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
}

// isHTMLGroup reports whether the group renders HTML and must therefore be
// dropped in headless mode.
func isHTMLGroup(g RouteGroup) bool {
	return g == GroupPublicUI || g == GroupProtectedUI || g == GroupAdmin
}
