package handler

import (
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type HomeHandler struct {
	messageService  *service.MessageService
	contactService  *service.ContactService
	securityService *service.SecurityService
	renderer        *web.Renderer
	version         string
}

func NewHomeHandler(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	secSvc *service.SecurityService,
	renderer *web.Renderer,
	version string,
) *HomeHandler {
	return &HomeHandler{
		messageService:  msgSvc,
		contactService:  contactSvc,
		securityService: secSvc,
		renderer:        renderer,
		version:         version,
	}
}

// ContributeRoutes registers the home dashboard route (protected).
func (h *HomeHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/", "Home dashboard", http.HandlerFunc(h.Show))
	return nil
}

func (h *HomeHandler) Show(w http.ResponseWriter, r *http.Request) {
	messageCount, _ := h.countMessages(r)
	contactCount, _ := h.contactService.CountContacts(r.Context())
	userCount, _ := h.securityService.CountUsers(r.Context())

	page := viewmodel.HomePage{
		MessageCount: messageCount,
		ContactCount: contactCount,
		UserCount:    userCount,
	}

	if err := h.renderer.RenderPage(w, r, "home", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *HomeHandler) countMessages(r *http.Request) (int64, error) {
	result, err := h.messageService.ListMessages(r.Context(), 1, 0)
	if err != nil {
		return 0, err
	}
	return result.Metadata.TotalItems, nil
}
