package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type SettingsHandler struct {
	securityService *service.SecurityService
	apiKeyService   *security.ApiKeyService
	renderer        *web.Renderer
}

func NewSettingsHandler(secSvc *service.SecurityService, apiKeySvc *security.ApiKeyService, renderer *web.Renderer) *SettingsHandler {
	return &SettingsHandler{
		securityService: secSvc,
		apiKeyService:   apiKeySvc,
		renderer:        renderer,
	}
}

func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings", h.Show)
	r.Post("/settings/profile", h.UpdateProfile)
	r.Post("/settings/preferences", h.UpdatePreferences)
	r.Post("/settings/api-keys", h.CreateApiKey)
	r.Post("/settings/api-keys/{id}/delete", h.DeleteApiKey)
}

func (h *SettingsHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}

	theme := ""
	if user.Theme != nil {
		theme = *user.Theme
	}

	language := ""
	if user.Language != nil {
		language = *user.Language
	}

	profile := viewmodel.ProfileData{
		Username:                  user.Username,
		Email:                     user.Email,
		AvatarURL:                 avatarURL,
		EmailNotificationsEnabled: user.EmailNotificationsEnabled,
		PushNotificationsEnabled:  user.PushNotificationsEnabled,
	}

	h.renderer.Render(w, "settings.html", viewmodel.SettingsPage{
		Profile:  profile,
		Theme:    theme,
		Language: language,
	})
}

func (h *SettingsHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	email := r.FormValue("email")

	var username *string
	if v := r.FormValue("username"); v != "" {
		username = &v
	}

	var avatarURL *string
	if v := r.FormValue("avatarUrl"); v != "" {
		avatarURL = &v
	}

	err := h.securityService.UpdateProfile(r.Context(), user.ID, email, username, avatarURL)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *SettingsHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	var language *string
	if v := r.FormValue("language"); v != "" {
		language = &v
	}

	var theme *string
	if v := r.FormValue("theme"); v != "" {
		theme = &v
	}

	var layout *string
	if v := r.FormValue("layout"); v != "" {
		layout = &v
	}

	err := h.securityService.UpdatePreferences(r.Context(), user.ID, language, theme, layout)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *SettingsHandler) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusBadRequest, "Use the API endpoint POST /api/v1/auth/api-keys instead")
}

func (h *SettingsHandler) DeleteApiKey(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	idStr := chi.URLParam(r, "id")
	keyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	err = h.apiKeyService.DeleteApiKey(r.Context(), user.ID, keyID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
