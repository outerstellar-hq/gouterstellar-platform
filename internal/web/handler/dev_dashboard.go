package handler

import (
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

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

// ContributeRoutes registers the dev dashboard admin routes. Registration is
// gated by the enabled flag so the routes only exist when DevDashboardEnabled
// is set in config.
func (h *DevDashboardHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	if !h.enabled {
		return nil
	}
	ctx.Routes.Admin(http.MethodGet, "/dev/dashboard", "Dev dashboard", http.HandlerFunc(h.Show))
	ctx.Routes.Admin(http.MethodPost, "/dev/outbox/process", "Process outbox", http.HandlerFunc(h.ProcessOutbox))
	ctx.Routes.Admin(http.MethodPost, "/dev/sessions/cleanup", "Cleanup sessions", http.HandlerFunc(h.CleanupSessions))
	ctx.Routes.Admin(http.MethodPost, "/dev/cache/invalidate", "Invalidate cache", http.HandlerFunc(h.InvalidateCache))
	return nil
}

func (h *DevDashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	if err := h.renderer.RenderPage(w, r, "dev_dashboard", nil); err != nil {
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
	h.messageService.InvalidateCache()
	writeJSON(w, http.StatusOK, map[string]string{"message": "Cache invalidated"})
}
