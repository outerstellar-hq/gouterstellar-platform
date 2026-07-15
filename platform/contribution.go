package platform

// NavigationItem is a single nav entry contributed by an extension.
type NavigationItem struct {
	Label string
	URL   string
	Icon  string
}

// NavigationRegistry collects navigation items during contribution.
type NavigationRegistry struct {
	items []NavigationItem
}

func NewNavigationRegistry() *NavigationRegistry {
	return &NavigationRegistry{}
}

func (n *NavigationRegistry) Add(label, url, icon string) {
	n.items = append(n.items, NavigationItem{Label: label, URL: url, Icon: icon})
}

func (n *NavigationRegistry) Items() []NavigationItem {
	return n.items
}

// ContributionContext is the capability surface passed to Extension.Contribute.
// Each instance is constructed per-extension and stamps the owner ID onto
// every route registration.
type ContributionContext struct {
	Routes     *RouteRegistry
	Navigation *NavigationRegistry
}

// NewContributionContext builds a context for a specific extension owner.
func NewContributionContext(owner string) *ContributionContext {
	return &ContributionContext{
		Routes:     newRouteRegistry(owner),
		Navigation: NewNavigationRegistry(),
	}
}

// RouteContributor contributes its own routes to the platform via the
// contribution context. Handlers implement this instead of a dead
// RegisterRoutes(chi.Router) method so the route declarations live next to
// the handlers they register and the core Extension no longer needs a
// pass-through Bundle of function values.
//
// The context carries the group-aware RouteRegistry (Public/Protected/API/
// Admin/Assets) and navigation registries; each contributor stamps
// its routes through ctx.Routes, which records the owner automatically.
type RouteContributor interface {
	ContributeRoutes(ctx *ContributionContext) error
}
