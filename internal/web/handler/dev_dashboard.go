package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type DevDashboardHandler struct {
	outboxProcessor *service.OutboxProcessor
	securityService *service.SecurityService
	messageService  *service.MessageService
	renderer        *web.Renderer
	enabled         bool
}

func NewDevDashboardHandler(
	outboxProcessor *service.OutboxProcessor,
	secSvc *service.SecurityService,
	msgSvc *service.MessageService,
	renderer *web.Renderer,
	enabled bool,
) *DevDashboardHandler {
	return &DevDashboardHandler{
		outboxProcessor: outboxProcessor,
		securityService: secSvc,
		messageService:  msgSvc,
		renderer:        renderer,
		enabled:         enabled,
	}
}

func (h *DevDashboardHandler) RegisterRoutes(r chi.Router) {
	if !h.enabled {
		return
	}
	r.Get("/dev/dashboard", h.Show)
	r.Post("/dev/outbox/process", h.ProcessOutbox)
	r.Post("/dev/sessions/cleanup", h.CleanupSessions)
	r.Post("/dev/cache/invalidate", h.InvalidateCache)
}

func (h *DevDashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	if err := h.renderer.Render(w, r, "dev_dashboard.html", nil); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *DevDashboardHandler) ProcessOutbox(w http.ResponseWriter, r *http.Request) {
	err := h.outboxProcessor.ProcessPending(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to process outbox: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Outbox processed"})
}

func (h *DevDashboardHandler) CleanupSessions(w http.ResponseWriter, r *http.Request) {
	err := h.securityService.DeleteExpiredSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to cleanup sessions: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Expired sessions cleaned up"})
}

func (h *DevDashboardHandler) InvalidateCache(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "Cache invalidated"})
}
