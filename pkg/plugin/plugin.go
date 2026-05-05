package plugin

type Plugin interface {
	Name() string
	Version() string
	Description() string
	Initialize() error
	Shutdown()
}

type PluginNavItem struct {
	Label    string
	URL      string
	Icon     string
	Children []PluginNavItem
}

type ServerPlugin interface {
	Plugin
	NavItems() []PluginNavItem
}

type DesktopPlugin interface {
	Plugin
}

type SharedPlugin interface {
	Plugin
}
