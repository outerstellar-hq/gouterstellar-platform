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

// AssetRegistry collects static asset declarations.
type AssetRegistry struct {
	entries []AssetEntry
}

type AssetEntry struct {
	Pattern string
	Path    string
}

func NewAssetRegistry() *AssetRegistry {
	return &AssetRegistry{}
}

func (a *AssetRegistry) Add(pattern, path string) {
	a.entries = append(a.entries, AssetEntry{Pattern: pattern, Path: path})
}

func (a *AssetRegistry) Entries() []AssetEntry {
	return a.entries
}

// AdminRegistry collects admin page contributions.
type AdminRegistry struct {
	pages []AdminPage
}

type AdminPage struct {
	Label string
	URL   string
}

func NewAdminRegistry() *AdminRegistry {
	return &AdminRegistry{}
}

func (a *AdminRegistry) Add(label, url string) {
	a.pages = append(a.pages, AdminPage{Label: label, URL: url})
}

func (a *AdminRegistry) Pages() []AdminPage {
	return a.pages
}

// ContributionContext is the capability surface passed to Extension.Contribute.
// Each instance is constructed per-extension and stamps the owner ID onto
// every route registration.
type ContributionContext struct {
	Routes     *RouteRegistry
	Navigation *NavigationRegistry
	Assets     *AssetRegistry
	Admin      *AdminRegistry
}

// NewContributionContext builds a context for a specific extension owner.
func NewContributionContext(owner string) *ContributionContext {
	return &ContributionContext{
		Routes:     newRouteRegistry(owner),
		Navigation: NewNavigationRegistry(),
		Assets:     NewAssetRegistry(),
		Admin:      NewAdminRegistry(),
	}
}
