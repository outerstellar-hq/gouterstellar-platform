package platform

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// buildRoutes mounts the given registrations onto a Chi router, skipping
// HTML groups in headless mode. It returns the slice of routes that were
// actually mounted. Registration is assumed to have already been validated.
func buildRoutes(r chi.Router, routes []RouteRegistration, mode PlatformMode, ownership map[string]RouteOwnership) []RouteRegistration {
	var mounted []RouteRegistration

	for _, reg := range routes {
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

// isHTMLGroup reports whether the group renders HTML and must therefore be
// dropped in headless mode.
func isHTMLGroup(g RouteGroup) bool {
	return g == GroupPublicUI || g == GroupProtectedUI || g == GroupAdmin
}
