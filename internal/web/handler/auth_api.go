package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type AuthAPI struct {
	securityService  *service.SecurityService
	apiKeyService    *security.ApiKeyService
	passwordResetSvc *service.PasswordResetService
	sessionSecure    bool
	analytics        service.AnalyticsService
	jwtService       *security.JwtService
}

func NewAuthAPI(
	secSvc *service.SecurityService,
	apiKeySvc *security.ApiKeyService,
	passwordResetSvc *service.PasswordResetService,
	sessionSecure bool,
	analytics service.AnalyticsService,
	jwtSvc *security.JwtService,
) *AuthAPI {
	return &AuthAPI{
		securityService:  secSvc,
		apiKeyService:    apiKeySvc,
		passwordResetSvc: passwordResetSvc,
		sessionSecure:    sessionSecure,
		analytics:        analytics,
		jwtService:       jwtSvc,
	}
}

func (h *AuthAPI) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/auth/login", h.Login)
	r.Post("/api/v1/auth/token", h.IssueToken)
	r.Post("/api/v1/auth/register", h.Register)
	r.Post("/api/v1/auth/change-password", h.ChangePassword)
	r.Post("/api/v1/auth/reset-password", h.RequestPasswordReset)
	r.Post("/api/v1/auth/confirm-reset", h.ConfirmPasswordReset)
	r.Post("/api/v1/auth/logout", h.Logout)
	r.Get("/api/v1/auth/profile", h.GetProfile)
	r.Put("/api/v1/auth/profile", h.UpdateProfile)
	r.Put("/api/v1/auth/notification-preferences", h.UpdateNotificationPreferences)
	r.Delete("/api/v1/auth/account", h.DeleteAccount)
	r.Post("/api/v1/auth/api-keys", h.CreateApiKey)
	r.Get("/api/v1/auth/api-keys", h.ListApiKeys)
	r.Delete("/api/v1/auth/api-keys/{id}", h.DeleteApiKey)
}

func (h *AuthAPI) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.securityService.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))

	h.analytics.Track(r.Context(), "user_login", map[string]interface{}{
		"username": user.Username,
	})

	writeJSON(w, http.StatusOK, model.AuthTokenResponse{
		Token:    token,
		Username: user.Username,
		Role:     string(user.Role),
	})
}

func (h *AuthAPI) IssueToken(w http.ResponseWriter, r *http.Request) {
	if !h.jwtService.IsEnabled() {
		writeError(w, http.StatusNotImplemented, "JWT authentication is not enabled")
		return
	}

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.securityService.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := h.jwtService.GenerateToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"expires_in": 3600,
		"username":   user.Username,
		"role":       string(user.Role),
	})
}

func (h *AuthAPI) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.securityService.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))

	h.analytics.Track(r.Context(), "user_register", map[string]interface{}{
		"username": user.Username,
	})

	writeJSON(w, http.StatusCreated, model.AuthTokenResponse{
		Token:    token,
		Username: user.Username,
		Role:     string(user.Role),
	})
}

func (h *AuthAPI) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req model.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.securityService.ChangePassword(r.Context(), user.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "password_change", map[string]interface{}{
		"userID": user.ID.String(),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

func (h *AuthAPI) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req model.PasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err := h.passwordResetSvc.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "If an account with that email exists, a reset link has been sent"})
}

func (h *AuthAPI) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req model.PasswordResetConfirm
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.passwordResetSvc.ResetPassword(r.Context(), req.Token, req.NewPassword)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password reset successfully"})
}

func (h *AuthAPI) Logout(w http.ResponseWriter, r *http.Request) {
	if token := web.GetSessionToken(r); token != "" {
		user := web.UserFromRequest(r)
		if user != nil {
			uid := user.ID
			username := user.Username
			if err := h.securityService.Logout(r.Context(), token); err != nil {
				slog.Warn("Failed to delete session on logout", "error", err)
			}
			h.securityService.AuditLogout(r.Context(), &uid, &username)
		} else {
			if err := h.securityService.Logout(r.Context(), token); err != nil {
				slog.Warn("Failed to delete session on logout", "error", err)
			}
		}
	}
	http.SetCookie(w, web.ClearSessionCookie(h.sessionSecure))
	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

func (h *AuthAPI) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	avatarURL := user.AvatarURL
	if avatarURL == nil && user.Email != "" {
		gravatar := web.GravatarURL(user.Email, 80)
		avatarURL = &gravatar
	}

	writeJSON(w, http.StatusOK, model.UserProfileResponse{
		Username:                  user.Username,
		Email:                     user.Email,
		AvatarURL:                 avatarURL,
		EmailNotificationsEnabled: user.EmailNotificationsEnabled,
		PushNotificationsEnabled:  user.PushNotificationsEnabled,
	})
}

func (h *AuthAPI) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req model.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.securityService.UpdateProfile(r.Context(), user.ID, req.Email, req.Username, req.AvatarURL)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "profile_update", map[string]interface{}{
		"userID": user.ID.String(),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "Profile updated"})
}

func (h *AuthAPI) UpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req model.UpdateNotificationPrefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.securityService.UpdateNotificationPreferences(r.Context(), user.ID, req.EmailEnabled, req.PushEnabled)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Notification preferences updated"})
}

func (h *AuthAPI) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	err := h.securityService.DeleteAccount(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.SetCookie(w, web.ClearSessionCookie(h.sessionSecure))

	h.analytics.Track(r.Context(), "account_deleted", map[string]interface{}{
		"userID": user.ID.String(),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "Account deleted"})
}

func (h *AuthAPI) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req model.CreateApiKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.apiKeyService.CreateApiKey(r.Context(), user.ID, req.Name)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	h.analytics.Track(r.Context(), "api_key_created", map[string]interface{}{
		"userID": user.ID.String(),
		"name":   req.Name,
	})

	writeJSON(w, http.StatusCreated, result)
}

func (h *AuthAPI) ListApiKeys(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	keys, err := h.apiKeyService.ListApiKeys(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, keys)
}

func (h *AuthAPI) DeleteApiKey(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	keyIDStr := chi.URLParam(r, "id")
	keyID, err := strconv.ParseInt(keyIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	err = h.apiKeyService.DeleteApiKey(r.Context(), user.ID, keyID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "API key deleted"})
}
