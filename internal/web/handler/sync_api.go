package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
)

type SyncAPI struct {
	messageService *service.MessageService
	contactService *service.ContactService
	analytics      service.AnalyticsService
}

func NewSyncAPI(msgSvc *service.MessageService, contactSvc *service.ContactService, analytics service.AnalyticsService) *SyncAPI {
	return &SyncAPI{
		messageService: msgSvc,
		contactService: contactSvc,
		analytics:      analytics,
	}
}

func (h *SyncAPI) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/sync", h.PullMessages)
	r.Post("/api/v1/sync", h.PushMessages)
	r.Get("/api/v1/sync/contacts", h.PullContacts)
	r.Post("/api/v1/sync/contacts", h.PushContacts)
}

func (h *SyncAPI) PullMessages(w http.ResponseWriter, r *http.Request) {
	since := getInt64Param(r, "since", 0)

	result, err := h.messageService.GetChangesSince(r.Context(), since)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "sync_pull", map[string]interface{}{
		"since": since,
		"count": len(result.Messages),
	})

	writeJSON(w, http.StatusOK, result)
}

func (h *SyncAPI) PushMessages(w http.ResponseWriter, r *http.Request) {
	var req model.SyncPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.messageService.ProcessPushRequest(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "sync_push", map[string]interface{}{
		"applied":   result.AppliedCount,
		"conflicts": len(result.Conflicts),
	})

	writeJSON(w, http.StatusOK, result)
}

func (h *SyncAPI) PullContacts(w http.ResponseWriter, r *http.Request) {
	since := getInt64Param(r, "since", 0)

	result, err := h.contactService.GetChangesSince(r.Context(), since)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "sync_pull_contacts", map[string]interface{}{
		"since": since,
		"count": len(result.Contacts),
	})

	writeJSON(w, http.StatusOK, result)
}

func (h *SyncAPI) PushContacts(w http.ResponseWriter, r *http.Request) {
	var req model.SyncPushContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.contactService.ProcessPushRequest(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "sync_push_contacts", map[string]interface{}{
		"applied":   result.AppliedCount,
		"conflicts": len(result.Conflicts),
	})

	writeJSON(w, http.StatusOK, result)
}
