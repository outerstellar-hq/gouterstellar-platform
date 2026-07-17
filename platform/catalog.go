package platform

import "sync"

// ExtensionInfo is the runtime-safe summary of an assembled extension.
type ExtensionInfo struct {
	ID             string
	Label          string
	Mode           PlatformMode
	RouteCount     int
	MigrationCount int
	Ownership      RouteOwnership
}

// RouteInfo is the diagnostic form of a mounted route. It deliberately omits
// the handler so the catalog can be exposed without leaking implementation
// details.
type RouteInfo struct {
	Owner       string `json:"owner"`
	Group       string `json:"group"`
	Method      string `json:"method"`
	PathPattern string `json:"pathPattern"`
	Description string `json:"description"`
	HandlerKind string `json:"handlerKind"`
}

// Catalog holds the immutable result of platform assembly for diagnostics and
// the extension dashboard. Replace is called once before the handler is served.
type Catalog struct {
	mu         sync.RWMutex
	extensions []ExtensionInfo
	routes     []RouteInfo
	readiness  []ReadinessStatus
}

func NewCatalog() *Catalog { return &Catalog{} }

func (c *Catalog) replace(extensions []Extension, routes []RouteRegistration, readiness []ReadinessStatus) {
	if c == nil {
		return
	}
	routeCounts := make(map[string]int)
	infos := make([]RouteInfo, len(routes))
	for i, route := range routes {
		routeCounts[route.Owner]++
		infos[i] = RouteInfo{
			Owner: route.Owner, Group: string(route.Group), Method: route.Method,
			PathPattern: route.Pattern, Description: route.Description, HandlerKind: route.HandlerKind,
		}
	}
	extensionInfos := make([]ExtensionInfo, len(extensions))
	for i, extension := range extensions {
		manifest := extension.Manifest()
		extensionInfos[i] = ExtensionInfo{
			ID: manifest.ID, Label: manifest.Label, Mode: manifest.Mode,
			RouteCount: routeCounts[manifest.ID], MigrationCount: len(manifest.Migrations),
			Ownership: manifest.Ownership,
		}
	}

	c.mu.Lock()
	c.extensions = extensionInfos
	c.routes = infos
	c.readiness = append([]ReadinessStatus{}, readiness...)
	c.mu.Unlock()
}

func (c *Catalog) Extensions() []ExtensionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]ExtensionInfo{}, c.extensions...)
}

func (c *Catalog) Routes() []RouteInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]RouteInfo{}, c.routes...)
}

func (c *Catalog) Readiness() []ReadinessStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]ReadinessStatus{}, c.readiness...)
}
