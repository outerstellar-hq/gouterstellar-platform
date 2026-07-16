package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type AuthAPI struct {
	securityService  *service.SecurityService
	totpService      *service.TOTPService
	apiKeyService    *security.ApiKeyService
	passwordResetSvc *service.PasswordResetService
	sessionSecure    bool
	analytics        service.AnalyticsService
	jwtService       *security.JwtService
}

func NewAuthAPI(
	secSvc *service.SecurityService,
	totpSvc *service.TOTPService,
	apiKeySvc *security.ApiKeyService,
	passwordResetSvc *service.PasswordResetService,
	sessionSecure bool,
	analytics service.AnalyticsService,
	jwtSvc *security.JwtService,
) *AuthAPI {
	return &AuthAPI{
		securityService:  secSvc,
		totpService:      totpSvc,
		apiKeyService:    apiKeySvc,
		passwordResetSvc: passwordResetSvc,
		sessionSecure:    sessionSecure,
		analytics:        analytics,
		jwtService:       jwtSvc,
	}
}

// ContributeRoutes registers the auth API routes (bearer auth applied by builder).
func (h *AuthAPI) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/login", "API login", http.HandlerFunc(h.Login))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/totp/verify", "Verify TOTP challenge", http.HandlerFunc(h.VerifyTOTP))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/totp/setup", "Create TOTP setup", http.HandlerFunc(h.SetupTOTP))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/totp/confirm", "Confirm TOTP setup", http.HandlerFunc(h.ConfirmTOTP))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/totp/disable", "Disable TOTP", http.HandlerFunc(h.DisableTOTP))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/token", "Issue token", http.HandlerFunc(h.IssueToken))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/register", "API register", http.HandlerFunc(h.Register))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/change-password", "API change password", http.HandlerFunc(h.ChangePassword))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/reset-password", "Request password reset", http.HandlerFunc(h.RequestPasswordReset))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/confirm-reset", "Confirm password reset", http.HandlerFunc(h.ConfirmPasswordReset))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/logout", "API logout", http.HandlerFunc(h.Logout))
	ctx.Routes.API(http.MethodGet, "/api/v1/auth/profile", "Get profile", http.HandlerFunc(h.GetProfile))
	ctx.Routes.API(http.MethodPut, "/api/v1/auth/profile", "Update profile", http.HandlerFunc(h.UpdateProfile))
	ctx.Routes.API(http.MethodPut, "/api/v1/auth/notification-preferences", "Update notif prefs", http.HandlerFunc(h.UpdateNotificationPreferences))
	ctx.Routes.API(http.MethodDelete, "/api/v1/auth/account", "Delete account", http.HandlerFunc(h.DeleteAccount))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/api-keys", "Create API key", http.HandlerFunc(h.CreateApiKey))
	ctx.Routes.API(http.MethodGet, "/api/v1/auth/api-keys", "List API keys", http.HandlerFunc(h.ListApiKeys))
	ctx.Routes.API(http.MethodDelete, "/api/v1/auth/api-keys/{id}", "Delete API key", http.HandlerFunc(h.DeleteApiKey))
	return nil
}

func (h *AuthAPI) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.securityService.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if required, ok := result.(model.TOTPRequired); ok {
		writeJSON(w, http.StatusUnauthorized, model.AuthTokenResponse{Status: "totp_required", PartialToken: required.PartialToken})
		return
	}
	authenticated, ok := result.(model.Authenticated)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	user := authenticated.User
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
		Status:   "success",
		Token:    token,
		Username: user.Username,
		Role:     string(user.Role),
	})
}

func (h *AuthAPI) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	var req model.TOTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	user, err := h.totpService.VerifyChallenge(r.Context(), req.PartialToken, req.Code)
	if err != nil {
		status := "invalid_code"
		if errors.Is(err, service.ErrTOTPChallengeExpired) {
			status = "expired"
		} else if errors.Is(err, service.ErrTOTPAccountLocked) {
			status = "locked"
		}
		writeJSON(w, http.StatusUnauthorized, model.TOTPVerifyResponse{Status: status})
		return
	}
	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}
	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))
	writeJSON(w, http.StatusOK, model.TOTPVerifyResponse{
		Status:   "success",
		Token:    token,
		Username: user.Username,
		Role:     string(user.Role),
	})
}

func (h *AuthAPI) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if user.TOTPEnabled {
		writeError(w, http.StatusConflict, "Two-factor authentication is already enabled")
		return
	}
	accountName := user.Email
	if accountName == "" {
		accountName = user.Username
	}
	setup, err := h.totpService.GenerateSetup(accountName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create TOTP setup")
		return
	}
	writeJSON(w, http.StatusOK, model.TOTPSetupResponse{Secret: setup.Secret, QRDataURI: setup.QRDataURI})
}

func (h *AuthAPI) ConfirmTOTP(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if user.TOTPEnabled {
		writeError(w, http.StatusConflict, "Two-factor authentication is already enabled")
		return
	}
	var req model.TOTPConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	backupCodes, err := h.totpService.ConfirmEnrollment(r.Context(), user.ID, req.Secret, req.Code)
	if errors.Is(err, service.ErrTOTPInvalidCode) {
		writeJSON(w, http.StatusOK, model.TOTPConfirmResponse{Status: "invalid_code"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to enable two-factor authentication")
		return
	}
	writeJSON(w, http.StatusCreated, model.TOTPConfirmResponse{Status: "success", BackupCodes: backupCodes})
}

func (h *AuthAPI) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req model.TOTPDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.totpService.Disable(r.Context(), user.ID, req.Password); err != nil {
		if errors.Is(err, service.ErrInvalidPassword) {
			writeError(w, http.StatusUnauthorized, "Invalid password")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to disable two-factor authentication")
		return
	}
	w.WriteHeader(http.StatusOK)
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

	result, err := h.securityService.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	authenticated, ok := result.(model.Authenticated)
	if !ok {
		writeError(w, http.StatusUnauthorized, "TOTP verification is required before issuing a token")
		return
	}
	user := authenticated.User
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
		var uid *uuid.UUID
		var username *string
		if user := web.UserFromRequest(r); user != nil {
			id := user.ID
			name := user.Username
			uid = &id
			username = &name
		}
		if err := h.securityService.Logout(r.Context(), token, uid, username); err != nil {
			slog.Warn("Failed to delete session on logout", "error", err)
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
		TOTPEnabled:               user.TOTPEnabled,
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
