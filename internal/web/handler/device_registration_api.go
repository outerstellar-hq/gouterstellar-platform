package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

var supportedDevicePlatforms = map[string]bool{"android": true, "ios": true}

type DeviceRegistrationAPI struct {
	deviceTokenRepo persistence.DeviceTokenRepository
}

func NewDeviceRegistrationAPI(repo persistence.DeviceTokenRepository) *DeviceRegistrationAPI {
	return &DeviceRegistrationAPI{deviceTokenRepo: repo}
}

// ContributeRoutes registers the device registration API routes (bearer auth applied by builder).
func (h *DeviceRegistrationAPI) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.API(http.MethodPost, "/api/v1/devices/register", "Register device", http.HandlerFunc(h.Register))
	ctx.Routes.API(http.MethodDelete, "/api/v1/devices/register", "Unregister device by token", http.HandlerFunc(h.UnregisterByToken))
	ctx.Routes.API(http.MethodDelete, "/api/v1/devices/{id}", "Unregister device", http.HandlerFunc(h.Unregister))
	return nil
}

type unregisterDeviceRequest struct {
	Token string `json:"token"`
}

func (h *DeviceRegistrationAPI) UnregisterByToken(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req unregisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Token = r.URL.Query().Get("token")
	}
	if strings.TrimSpace(req.Token) == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	deleted, err := h.deviceTokenRepo.DeleteDeviceTokenByValue(r.Context(), req.Token, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if deleted == 0 {
		writeError(w, http.StatusForbidden, "Token not found or not owned by this user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

	if !supportedDevicePlatforms[req.Platform] {
		writeError(w, http.StatusBadRequest, "platform must be one of: android, ios")
		return
	}
	if strings.TrimSpace(req.Token) == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	_, err := h.deviceTokenRepo.UpsertDeviceToken(r.Context(), user.ID, req.Platform, req.Token, req.AppBundle)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
