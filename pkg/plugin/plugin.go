package plugin

type Plugin interface {
	Name() string
	Version() string
	Description() string
	Initialize() error
	Shutdown()
}

type ServerPlugin interface {
	Plugin
}

type DesktopPlugin interface {
	Plugin
}

type SharedPlugin interface {
	Plugin
}
