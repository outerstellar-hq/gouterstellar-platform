package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type NotificationAPI struct {
	notificationService *service.NotificationService
}

type notificationDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Type      string `json:"type"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

func NewNotificationAPI(notifSvc *service.NotificationService) *NotificationAPI {
	return &NotificationAPI{notificationService: notifSvc}
}

// ContributeRoutes registers the notification API routes (bearer auth applied by builder).
func (h *NotificationAPI) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.API(http.MethodGet, "/api/v1/notifications", "List notifications", http.HandlerFunc(h.List))
	ctx.Routes.API(http.MethodGet, "/api/v1/notifications/unread-count", "Unread count", http.HandlerFunc(h.UnreadCount))
	ctx.Routes.API(http.MethodPut, "/api/v1/notifications/{id}/read", "Mark read", http.HandlerFunc(h.MarkRead))
	ctx.Routes.API(http.MethodPut, "/api/v1/notifications/read-all", "Mark all read", http.HandlerFunc(h.MarkAllRead))
	ctx.Routes.API(http.MethodDelete, "/api/v1/notifications/{id}", "Delete notification", http.HandlerFunc(h.Delete))
	return nil
}

func (h *NotificationAPI) List(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 50)
	offset := (page - 1) * pageSize

	notifications, err := h.notificationService.ListForUser(r.Context(), user.ID, safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	result := make([]notificationDTO, len(notifications))
	for i, notification := range notifications {
		result[i] = notificationDTO{
			ID:        notification.ID.String(),
			Title:     notification.Title,
			Body:      notification.Body,
			Type:      notification.Type,
			Read:      notification.IsRead(),
			CreatedAt: notification.CreatedAt.Format(time.RFC3339Nano),
		}
	}
	writeJSON(w, http.StatusOK, result)
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

	w.WriteHeader(http.StatusNoContent)
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

	w.WriteHeader(http.StatusNoContent)
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
