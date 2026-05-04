package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type DeviceRegistrationAPI struct {
	deviceTokenRepo persistence.DeviceTokenRepository
}

func NewDeviceRegistrationAPI(repo persistence.DeviceTokenRepository) *DeviceRegistrationAPI {
	return &DeviceRegistrationAPI{deviceTokenRepo: repo}
}

func (h *DeviceRegistrationAPI) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/devices/register", h.Register)
	r.Delete("/api/v1/devices/{id}", h.Unregister)
}

type registerDeviceRequest struct {
	Platform  string  `json:"platform"`
	Token     string  `json:"token"`
	AppBundle *string `json:"appBundle"`
}

func (h *DeviceRegistrationAPI) Register(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req registerDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Platform == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "Platform and token are required")
		return
	}

	_, err := h.deviceTokenRepo.UpsertDeviceToken(r.Context(), user.ID, req.Platform, req.Token, req.AppBundle)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Device registered"})
}

func (h *DeviceRegistrationAPI) Unregister(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	deleted, err := h.deviceTokenRepo.DeleteDeviceToken(r.Context(), id, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if deleted == 0 {
		writeError(w, http.StatusNotFound, "Device not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Device unregistered"})
}
