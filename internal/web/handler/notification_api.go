package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type NotificationAPI struct {
	notificationService *service.NotificationService
}

func NewNotificationAPI(notifSvc *service.NotificationService) *NotificationAPI {
	return &NotificationAPI{notificationService: notifSvc}
}

func (h *NotificationAPI) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/notifications", h.List)
	r.Get("/api/v1/notifications/unread-count", h.UnreadCount)
	r.Put("/api/v1/notifications/{id}/read", h.MarkRead)
	r.Put("/api/v1/notifications/read-all", h.MarkAllRead)
	r.Delete("/api/v1/notifications/{id}", h.Delete)
}

func (h *NotificationAPI) List(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	notifications, err := h.notificationService.ListForUser(r.Context(), user.ID, safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, notifications)
}

func (h *NotificationAPI) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	count, err := h.notificationService.CountUnread(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"count": count})
}

func (h *NotificationAPI) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
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

	writeJSON(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

func (h *NotificationAPI) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	err := h.notificationService.MarkAllRead(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}

func (h *NotificationAPI) Delete(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
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

	writeJSON(w, http.StatusOK, map[string]string{"message": "Notification deleted"})
}
