package handler

import (
	"net/http"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

type ExtensionAdminHandler struct {
	catalog  *extplatform.Catalog
	renderer *web.Renderer
}

func NewExtensionAdminHandler(catalog *extplatform.Catalog, renderer *web.Renderer) *ExtensionAdminHandler {
	return &ExtensionAdminHandler{catalog: catalog, renderer: renderer}
}

func (h *ExtensionAdminHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Admin(http.MethodGet, "/admin", "Admin index", http.HandlerFunc(h.Index))
	ctx.Routes.Admin(http.MethodGet, "/admin/extensions", "Extension dashboard", http.HandlerFunc(h.Extensions))
	return nil
}

func (h *ExtensionAdminHandler) Index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/extensions", http.StatusFound)
}

func (h *ExtensionAdminHandler) Extensions(w http.ResponseWriter, r *http.Request) {
	extensions := h.catalog.Extensions()
	routes := routesByOwner(h.catalog.Routes())
	readiness := readinessByOwner(h.catalog.Readiness())
	cards := make([]viewmodel.ExtensionCard, len(extensions))
	for i, extension := range extensions {
		cards[i] = viewmodel.ExtensionCard{
			ID: extension.ID, Label: extension.Label, Mode: string(extension.Mode),
			RouteCount: extension.RouteCount, MigrationCount: extension.MigrationCount,
			UIOwnership: extension.Ownership.UI, APIOwnership: extension.Ownership.API,
			AdminOwnership: extension.Ownership.Admin, AssetOwnership: extension.Ownership.Assets,
			Routes: routes[extension.ID], Readiness: readiness[extension.ID],
		}
	}
	if err := h.renderer.RenderPage(w, r, "admin_extensions", viewmodel.ExtensionsPage{Extensions: cards}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func routesByOwner(routes []extplatform.RouteInfo) map[string][]viewmodel.ExtensionRoute {
	byOwner := make(map[string][]viewmodel.ExtensionRoute)
	for _, route := range routes {
		byOwner[route.Owner] = append(byOwner[route.Owner], viewmodel.ExtensionRoute{
			Method: route.Method, PathPattern: route.PathPattern, Group: route.Group,
			Description: route.Description, HandlerKind: route.HandlerKind,
		})
	}
	return byOwner
}

func readinessByOwner(statuses []extplatform.ReadinessStatus) map[string][]viewmodel.ExtensionReadiness {
	byOwner := make(map[string][]viewmodel.ExtensionReadiness)
	for _, status := range statuses {
		byOwner[status.Owner] = append(byOwner[status.Owner], viewmodel.ExtensionReadiness{
			Name: status.Name, Status: status.Status, Message: status.Message,
		})
	}
	return byOwner
}
