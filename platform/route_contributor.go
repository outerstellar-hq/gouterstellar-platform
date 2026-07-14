package platform

// RouteContributor contributes its own routes to the platform via the
// contribution context. Handlers implement this instead of a dead
// RegisterRoutes(chi.Router) method so the route declarations live next to
// the handlers they register and the core Extension no longer needs a
// pass-through Bundle of function values.
//
// The context carries the group-aware RouteRegistry (Public/Protected/API/
// Admin/Assets) and navigation/admin registries; each contributor stamps
// its routes through ctx.Routes, which records the owner automatically.
type RouteContributor interface {
	ContributeRoutes(ctx *ContributionContext) error
}
