package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type HomeHandler struct {
	messageService *service.MessageService
	contactService *service.ContactService
	securityService *service.SecurityService
	renderer       *web.Renderer
	version        string
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

func (h *HomeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.Show)
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

	h.renderer.Render(w, "home.html", page)
}

func (h *HomeHandler) countMessages(r *http.Request) (int64, error) {
	result, err := h.messageService.ListMessages(r.Context(), 1, 0)
	if err != nil {
		return 0, err
	}
	return result.Metadata.TotalItems, nil
}
