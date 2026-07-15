package platform

import "net/http"

// RouteGroup categorises a route so the wire root can apply the right
// middleware and so headless mode knows which groups to drop.
type RouteGroup string

const (
	GroupPublicUI    RouteGroup = "public-ui"
	GroupProtectedUI RouteGroup = "protected-ui"
	GroupAPI         RouteGroup = "api"
	GroupAdmin       RouteGroup = "admin"
	GroupAssets      RouteGroup = "assets"
)

// RouteRegistration is a single route collected from an extension's
// Contribute callback. The Owner field is stamped from the registry.
type RouteRegistration struct {
	Owner       string
	Method      string
	Pattern     string
	Group       RouteGroup
	Description string
	Handler     http.Handler
}

// RouteRegistry collects route registrations for a single extension owner.
// Each helper method stamps the owner so extensions never set it by hand.
type RouteRegistry struct {
	owner  string
	routes []RouteRegistration
}

func newRouteRegistry(owner string) *RouteRegistry {
	return &RouteRegistry{owner: owner}
}

func (r *RouteRegistry) Public(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupPublicUI)
}

func (r *RouteRegistry) Protected(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupProtectedUI)
}

func (r *RouteRegistry) API(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupAPI)
}

func (r *RouteRegistry) Admin(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupAdmin)
}

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

// All returns every registration collected so far.
func (r *RouteRegistry) All() []RouteRegistration {
	return r.routes
}

// ByOwner filters the collected routes to those owned by the given id.
func (r *RouteRegistry) ByOwner(id string) []RouteRegistration {
	var result []RouteRegistration
	for _, r := range r.routes {
		if r.Owner == id {
			result = append(result, r)
		}
	}
	return result
}

// Find looks up a registration by method and pattern.
func (r *RouteRegistry) Find(method, pattern string) (RouteRegistration, bool) {
	for _, reg := range r.routes {
		if reg.Method == method && reg.Pattern == pattern {
			return reg, true
		}
	}
	return RouteRegistration{}, false
}
