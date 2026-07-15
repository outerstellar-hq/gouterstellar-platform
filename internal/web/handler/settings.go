package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

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

// ContributeRoutes registers the settings UI routes (protected).
func (h *SettingsHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/settings", "Settings page", http.HandlerFunc(h.Show))
	ctx.Routes.Protected(http.MethodPost, "/settings/profile", "Update profile", http.HandlerFunc(h.UpdateProfile))
	ctx.Routes.Protected(http.MethodPost, "/settings/password", "Change password", http.HandlerFunc(h.ChangePassword))
	ctx.Routes.Protected(http.MethodPost, "/settings/preferences", "Update preferences", http.HandlerFunc(h.UpdatePreferences))
	ctx.Routes.Protected(http.MethodPost, "/settings/api-keys", "Create API key", http.HandlerFunc(h.CreateApiKey))
	ctx.Routes.Protected(http.MethodPost, "/settings/api-keys/{id}/delete", "Delete API key", http.HandlerFunc(h.DeleteApiKey))
	ctx.Routes.Protected(http.MethodPost, "/settings/notifications", "Update notification prefs", http.HandlerFunc(h.UpdateNotificationPrefs))
	ctx.Routes.Protected(http.MethodGet, "/settings/sessions", "Active sessions", http.HandlerFunc(h.Sessions))
	ctx.Routes.Protected(http.MethodPost, "/settings/sessions/{tokenHash}/revoke", "Revoke session", http.HandlerFunc(h.RevokeSession))
	return nil
}

func (h *SettingsHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	activeTab := r.URL.Query().Get("tab")
	if activeTab == "" {
		activeTab = "profile"
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

	layout := ""
	if user.Layout != nil {
		layout = *user.Layout
	}

	profile := viewmodel.ProfileData{
		Username:                  user.Username,
		Email:                     user.Email,
		AvatarURL:                 avatarURL,
		EmailNotificationsEnabled: user.EmailNotificationsEnabled,
		PushNotificationsEnabled:  user.PushNotificationsEnabled,
	}

	apiKeySummaries, _ := h.apiKeyService.ListApiKeys(r.Context(), user.ID)
	apiKeys := make([]viewmodel.ApiKeyItem, 0, len(apiKeySummaries))
	for _, k := range apiKeySummaries {
		lastUsed := ""
		if k.LastUsedAt != nil {
			lastUsed = *k.LastUsedAt
		}
		apiKeys = append(apiKeys, viewmodel.ApiKeyItem{
			ID:        k.ID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			CreatedAt: k.CreatedAt,
			LastUsed:  lastUsed,
		})
	}

	newApiKey := r.URL.Query().Get("new_key")

	if err := h.renderer.RenderPage(w, r, "settings", viewmodel.SettingsPage{
		ActiveTab: activeTab,
		Profile:   profile,
		ApiKeys:   apiKeys,
		Theme:     theme,
		Language:  language,
		Layout:    layout,
		NewApiKey: newApiKey,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
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

	http.Redirect(w, r, "/settings?tab=profile", http.StatusSeeOther)
}

func (h *SettingsHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	currentPassword := r.FormValue("currentPassword")
	newPassword := r.FormValue("newPassword")
	confirmPassword := r.FormValue("confirmPassword")

	if newPassword != confirmPassword {
		writeError(w, http.StatusBadRequest, "New password and confirmation do not match")
		return
	}

	err := h.securityService.ChangePassword(r.Context(), user.ID, currentPassword, newPassword)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings?tab=password", http.StatusSeeOther)
}

func (h *SettingsHandler) UpdateNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	emailEnabled := r.FormValue("email_notifications") == "on"
	pushEnabled := r.FormValue("push_notifications") == "on"

	err := h.securityService.UpdateNotificationPreferences(r.Context(), user.ID, emailEnabled, pushEnabled)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings?tab=notifications", http.StatusSeeOther)
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

	http.Redirect(w, r, "/settings?tab=appearance", http.StatusSeeOther)
}

func (h *SettingsHandler) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Key name is required")
		return
	}

	resp, err := h.apiKeyService.CreateApiKey(r.Context(), user.ID, name)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings?tab=api-keys&new_key="+url.QueryEscape(resp.Key), http.StatusSeeOther) // #nosec G710 -- resp.Key is a freshly generated API key, not user input
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

	http.Redirect(w, r, "/settings?tab=api-keys", http.StatusSeeOther)
}

// Sessions lists the user's active (non-expired) sessions for management.
func (h *SettingsHandler) Sessions(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	sessions, err := h.securityService.ListUserSessions(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	items := make([]viewmodel.SessionItem, len(sessions))
	for i, s := range sessions {
		items[i] = viewmodel.SessionItem{
			TokenHash:       s.TokenHash,
			MaskedTokenHash: s.MaskedTokenHash(),
			CreatedAt:       s.CreatedAt.Format("2006-01-02 15:04"),
			ExpiresAt:       s.ExpiresAt.Format("2006-01-02 15:04"),
		}
	}

	if err := h.renderer.RenderPage(w, r, "settings_sessions", viewmodel.SettingsSessionsPage{Sessions: items}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// RevokeSession deletes a single session by its token hash (read from the
// {tokenHash} URL parameter), then redirects back to the sessions list.
func (h *SettingsHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	tokenHash := chi.URLParam(r, "tokenHash")
	if err := h.securityService.RevokeSession(r.Context(), tokenHash); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/settings/sessions", http.StatusSeeOther)
}
