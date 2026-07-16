package handler

import (
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
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
	cards := make([]viewmodel.ExtensionCard, len(extensions))
	for i, extension := range extensions {
		cards[i] = viewmodel.ExtensionCard{
			ID: extension.ID, Label: extension.Label, Mode: string(extension.Mode),
			RouteCount: extension.RouteCount, MigrationCount: extension.MigrationCount,
		}
	}
	if err := h.renderer.RenderPage(w, r, "admin_extensions", viewmodel.ExtensionsPage{Extensions: cards}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
