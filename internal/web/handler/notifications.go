package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type NotificationsHandler struct {
	notificationService *service.NotificationService
	renderer            *web.Renderer
}

func NewNotificationsHandler(notifSvc *service.NotificationService, renderer *web.Renderer) *NotificationsHandler {
	return &NotificationsHandler{
		notificationService: notifSvc,
		renderer:            renderer,
	}
}

func (h *NotificationsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/notifications", h.List)
	r.Post("/notifications/{id}/read", h.MarkRead)
	r.Post("/notifications/read-all", h.MarkAllRead)
	r.Post("/notifications/{id}/delete", h.Delete)
}

func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	notifications, err := h.notificationService.ListForUser(r.Context(), user.ID, int32(pageSize), int32(offset))
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, "error.html", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Failed to load notifications",
		}, http.StatusInternalServerError)
		return
	}

	unreadCount, _ := h.notificationService.CountUnread(r.Context(), user.ID)

	items := make([]viewmodel.NotificationItem, len(notifications))
	for i, n := range notifications {
		items[i] = viewmodel.NotificationItem{
			ID:        n.ID.String(),
			Title:     n.Title,
			Body:      n.Body,
			Type:      n.Type,
			Read:      n.IsRead(),
			CreatedAt: n.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	if err := h.renderer.Render(w, "notifications.html", viewmodel.NotificationsPage{
		Notifications: items,
		UnreadCount:   int(unreadCount),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.MarkRead(r.Context(), id, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	err := h.notificationService.MarkAllRead(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

func (h *NotificationsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.Delete(r.Context(), id, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}
