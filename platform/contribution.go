package platform

import (
	"fmt"
	"io/fs"
	"net/http"
)

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
	Pages      *PageRegistry
	Operations *OperationsRegistry
	Banners    *BannerRegistry
}

// NewContributionContext builds a context for a specific extension owner.
func NewContributionContext(owner string) *ContributionContext {
	return newContributionContext(owner, ServiceBag{}, assetHostOptions{})
}

func newContributionContext(owner string, services ServiceBag, assets assetHostOptions) *ContributionContext {
	routes := newRouteRegistry(owner, assets)
	return &ContributionContext{
		Routes:     routes,
		Navigation: NewNavigationRegistry(),
		Pages:      &PageRegistry{owner: owner, renderer: services.Pages},
		Operations: newOperationsRegistry(owner, routes, services.Pages, services.OperationsAudit),
		Banners:    &BannerRegistry{owner: owner},
	}
}

// TemplateSource describes extension-owned templates. PagesDir contains page
// files keyed by filename; PartialsDir is optional and contains templates used
// only by this extension's pages.
type TemplateSource struct {
	FS          fs.FS
	PagesDir    string
	PartialsDir string
}

// PageRegistry stamps template registrations with the contributing extension
// owner and renders only through the platform's shared page renderer.
type PageRegistry struct {
	owner    string
	renderer PageRenderer
}

// Register parses and validates all templates immediately. A returned error
// aborts platform assembly before the first request is served.
func (p *PageRegistry) Register(source TemplateSource) error {
	if p == nil || p.renderer == nil {
		return fmt.Errorf("page rendering capability is not configured")
	}
	if source.FS == nil {
		return fmt.Errorf("extension %s template filesystem is nil", p.owner)
	}
	return p.renderer.RegisterTemplates(p.owner, source.FS, source.PagesDir, source.PartialsDir)
}

// Render renders a registered extension page inside the shared shell.
func (p *PageRegistry) Render(w http.ResponseWriter, req *http.Request, page string, data any) error {
	if p == nil || p.renderer == nil {
		return fmt.Errorf("page rendering capability is not configured")
	}
	return p.renderer.RenderPage(w, req, page, data)
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
