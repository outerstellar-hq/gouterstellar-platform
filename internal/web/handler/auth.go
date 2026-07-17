package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

type AuthHandler struct {
	securityService    *service.SecurityService
	totpService        *service.TOTPService
	passwordResetSvc   *service.PasswordResetService
	renderer           *web.Renderer
	sessionSecure      bool
	analytics          service.AnalyticsService
	googleLoginEnabled bool
	appleLoginEnabled  bool
}

func NewAuthHandler(
	secSvc *service.SecurityService,
	totpSvc *service.TOTPService,
	passwordResetSvc *service.PasswordResetService,
	renderer *web.Renderer,
	sessionSecure bool,
	analytics service.AnalyticsService,
	googleLoginEnabled bool,
	appleLoginEnabled bool,
) *AuthHandler {
	return &AuthHandler{
		securityService:    secSvc,
		totpService:        totpSvc,
		passwordResetSvc:   passwordResetSvc,
		renderer:           renderer,
		sessionSecure:      sessionSecure,
		analytics:          analytics,
		googleLoginEnabled: googleLoginEnabled,
		appleLoginEnabled:  appleLoginEnabled,
	}
}

// ContributeRoutes registers the auth UI routes (public, no auth required).
func (h *AuthHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Public(http.MethodGet, "/auth", "Login page", http.HandlerFunc(h.ShowLogin))
	ctx.Routes.Public(http.MethodGet, "/auth/components/forms/{mode}", "Auth form fragment", http.HandlerFunc(h.AuthForm))
	ctx.Routes.Public(http.MethodPost, "/auth/components/result", "Process auth form", http.HandlerFunc(h.HandleAuthResult))
	ctx.Routes.Public(http.MethodPost, "/auth/login", "Handle login", http.HandlerFunc(h.HandleLogin))
	ctx.Routes.Public(http.MethodPost, "/auth/totp/verify", "Verify TOTP challenge", http.HandlerFunc(h.HandleTOTPVerify))
	ctx.Routes.Public(http.MethodPost, "/auth/components/totp-verify", "Verify TOTP challenge component", http.HandlerFunc(h.HandleTOTPVerifyComponent))
	ctx.Routes.Public(http.MethodPost, "/auth/register", "Handle registration", http.HandlerFunc(h.HandleRegister))
	ctx.Routes.Public(http.MethodGet, "/auth/reset", "Reset password page", http.HandlerFunc(h.ShowResetPassword))
	ctx.Routes.Public(http.MethodPost, "/auth/reset", "Handle password reset", http.HandlerFunc(h.HandleResetPassword))
	ctx.Routes.Public(http.MethodGet, "/auth/reset/{token}", "Password reset token page", http.HandlerFunc(h.ShowResetToken))
	ctx.Routes.Public(http.MethodPost, "/auth/components/reset-confirm", "Confirm password reset", http.HandlerFunc(h.HandleConfirmResetPasswordComponent))
	ctx.Routes.Public(http.MethodGet, "/auth/reset/confirm", "Confirm reset password page", http.HandlerFunc(h.ShowConfirmResetPassword))
	ctx.Routes.Public(http.MethodPost, "/auth/reset/confirm", "Handle password reset confirmation", http.HandlerFunc(h.HandleConfirmResetPassword))
	ctx.Routes.Protected(http.MethodPost, "/logout", "Logout", http.HandlerFunc(h.HandleLogout))
	ctx.Routes.Protected(http.MethodGet, "/auth/change-password", "Change password page", http.HandlerFunc(h.ShowChangePassword))
	ctx.Routes.Protected(http.MethodPost, "/auth/components/change-password", "Handle password change", http.HandlerFunc(h.HandleChangePasswordComponent))
	ctx.Routes.Protected(http.MethodPost, "/auth/change-password", "Handle password change", http.HandlerFunc(h.HandleChangePassword))
	return nil
}

func (h *AuthHandler) AuthForm(w http.ResponseWriter, r *http.Request) {
	mode := chi.URLParam(r, "mode")
	switch mode {
	case "register", "recover", "sign-in":
	default:
		mode = "sign-in"
	}
	if err := h.renderer.RenderPartial(w, "auth_form", authFormFragment{
		Mode:                mode,
		ReturnTo:            safeReturnTo(r.URL.Query().Get("returnTo")),
		CSRFToken:           web.CSRFTokenFromRequest(r),
		RegistrationEnabled: h.securityService.RegistrationEnabled(),
		AppleLoginEnabled:   h.appleLoginEnabled,
		Language:            web.LanguageFromRequest(r),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) HandleAuthResult(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAuthError(w, r, "Invalid form submission")
		return
	}
	username := r.FormValue("email")
	password := r.FormValue("password")
	returnTo := safeReturnTo(r.URL.Query().Get("returnTo"))
	if returnTo == "/" {
		returnTo = safeReturnTo(r.FormValue("returnTo"))
	}
	switch r.FormValue("mode") {
	case "", "sign-in":
		result, err := h.securityService.Authenticate(r.Context(), username, password)
		if err != nil {
			h.renderAuthResult(w, false, "Sign in failed", "Invalid credentials")
			return
		}
		switch authenticated := result.(type) {
		case model.Authenticated:
			token, err := h.securityService.CreateSession(r.Context(), authenticated.User.ID)
			if err != nil {
				h.renderAuthResult(w, false, "Sign in failed", "Failed to create session")
				return
			}
			http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))
			// #nosec G710 -- safeReturnTo permits only same-origin absolute paths and rejects scheme-relative/backslash forms.
			http.Redirect(w, r, returnTo, http.StatusFound)
		case model.TOTPRequired:
			if err := h.renderer.RenderPartial(w, "auth_totp", authTOTPFragment{
				PartialToken: authenticated.PartialToken,
				ReturnTo:     returnTo,
				CSRFToken:    web.CSRFTokenFromRequest(r),
				Language:     web.LanguageFromRequest(r),
			}); err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
			}
		default:
			h.renderAuthResult(w, false, "Sign in failed", "Invalid credentials")
		}
	case "register":
		if !h.securityService.RegistrationEnabled() {
			h.renderAuthResult(w, false, "Registration failed", "Registration is currently disabled")
			return
		}
		if password != r.FormValue("confirmPassword") {
			h.renderAuthResult(w, false, "Registration failed", "Password and confirmation do not match")
			return
		}
		if _, err := h.securityService.Register(r.Context(), username, password); err != nil {
			h.renderAuthResult(w, false, "Registration failed", registrationErrorMessage(err))
			return
		}
		http.Redirect(w, r, "/auth?registered=true", http.StatusFound)
	case "recover":
		if username != "" {
			_, _ = h.passwordResetSvc.RequestPasswordReset(r.Context(), username)
		}
		h.renderAuthResult(w, true, "Request accepted", "If an account exists, a reset link has been sent.")
	default:
		h.renderAuthResult(w, false, "Unable to continue", "Unknown authentication mode")
	}
}

type authFormFragment struct {
	Mode                string
	ReturnTo            string
	CSRFToken           string
	RegistrationEnabled bool
	AppleLoginEnabled   bool
	Language            string
}

type authResultFragment struct {
	Success bool
	Title   string
	Message string
}

type authTOTPFragment struct {
	PartialToken string
	ReturnTo     string
	CSRFToken    string
	Error        string
	Language     string
}

func (h *AuthHandler) renderAuthResult(w http.ResponseWriter, success bool, title, message string) {
	if err := h.renderer.RenderPartial(w, "auth_result", authResultFragment{Success: success, Title: title, Message: message}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("returnTo")
	registrationEnabled := h.securityService.RegistrationEnabled()
	registerRequested := r.URL.Query().Get("mode") == "register"
	page := viewmodel.AuthPage{
		ReturnTo:            returnTo,
		CSRFToken:           web.CSRFTokenFromRequest(r),
		GoogleLoginEnabled:  h.googleLoginEnabled,
		AppleLoginEnabled:   h.appleLoginEnabled,
		RegistrationEnabled: registrationEnabled,
		RegisterMode:        registerRequested && registrationEnabled,
	}
	if registerRequested && !registrationEnabled {
		page.Error = "Registration is currently disabled"
	}
	if err := h.renderer.RenderPage(w, r, "auth_login", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAuthError(w, r, "Invalid form submission")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	result, err := h.securityService.Authenticate(r.Context(), username, password)
	if err != nil {
		h.renderAuthError(w, r, "Invalid username or password")
		return
	}
	returnTo := r.FormValue("returnTo")
	switch authenticated := result.(type) {
	case model.Authenticated:
		h.completeLogin(w, r, authenticated.User, returnTo)
	case model.TOTPRequired:
		_ = h.renderer.RenderPage(w, r, "auth_login", viewmodel.AuthPage{
			ReturnTo:     returnTo,
			CSRFToken:    web.CSRFTokenFromRequest(r),
			TOTPRequired: true,
			PartialToken: authenticated.PartialToken,
		})
	default:
		h.renderAuthError(w, r, "Invalid username or password")
	}
}

func (h *AuthHandler) HandleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderTOTPError(w, r, "Invalid form submission")
		return
	}
	user, err := h.totpService.VerifyChallenge(r.Context(), r.FormValue("partialToken"), r.FormValue("code"))
	if err != nil {
		message := "Invalid authentication code"
		if errors.Is(err, service.ErrTOTPChallengeExpired) {
			message = "Your sign-in challenge expired. Enter your password again."
		} else if errors.Is(err, service.ErrTOTPAccountLocked) {
			message = "Your account is temporarily locked after too many failed attempts."
		}
		h.renderTOTPError(w, r, message)
		return
	}
	h.completeLogin(w, r, user, r.FormValue("returnTo"))
}

func (h *AuthHandler) HandleTOTPVerifyComponent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAuthResult(w, false, "Verification failed", "Invalid form submission")
		return
	}
	partialToken := r.FormValue("partialToken")
	user, err := h.totpService.VerifyChallenge(r.Context(), partialToken, r.FormValue("code"))
	if err != nil {
		message := "The authentication code was not valid."
		if errors.Is(err, service.ErrTOTPChallengeExpired) {
			message = "Your sign-in challenge has expired."
		} else if errors.Is(err, service.ErrTOTPAccountLocked) {
			message = "Your account is temporarily locked."
		}
		if renderErr := h.renderer.RenderPartial(w, "auth_totp", authTOTPFragment{
			PartialToken: partialToken,
			ReturnTo:     safeReturnTo(r.FormValue("returnTo")),
			CSRFToken:    web.CSRFTokenFromRequest(r),
			Error:        message,
			Language:     web.LanguageFromRequest(r),
		}); renderErr != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
		return
	}
	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.renderAuthResult(w, false, "Verification failed", "Failed to create session")
		return
	}
	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))
	w.Header().Set("HX-Redirect", safeReturnTo(r.FormValue("returnTo")))
	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) completeLogin(w http.ResponseWriter, r *http.Request, user *model.User, returnTo string) {
	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.renderAuthError(w, r, "Failed to create session")
		return
	}
	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))
	h.analytics.Track(r.Context(), "user_login", map[string]interface{}{"username": user.Username})
	if returnTo == "" || !isSafeRedirect(returnTo) {
		returnTo = "/"
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther) // #nosec G710 -- returnTo validated by isSafeRedirect above
}

func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAuthError(w, r, "Invalid form submission")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	if password != r.FormValue("confirmPassword") {
		h.renderAuthError(w, r, "Password and confirmation do not match")
		return
	}

	user, err := h.securityService.Register(r.Context(), username, password)
	if err != nil {
		h.renderAuthError(w, r, registrationErrorMessage(err))
		return
	}

	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.renderAuthError(w, r, "Failed to create session")
		return
	}

	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))

	h.analytics.Track(r.Context(), "user_register", map[string]interface{}{
		"username": user.Username,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func registrationErrorMessage(err error) string {
	var disabled *model.RegistrationDisabledError
	var weak *model.WeakPasswordError
	var duplicate *model.UsernameAlreadyExistsError
	var validation *model.ValidationError
	switch {
	case errors.As(err, &disabled), errors.As(err, &weak), errors.As(err, &duplicate):
		return err.Error()
	case errors.As(err, &validation):
		return strings.Join(validation.Errors, ". ")
	default:
		slog.Error("Registration failed", "error", err)
		return "Registration failed. Please try again."
	}
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/auth", http.StatusSeeOther)
}

func (h *AuthHandler) ShowChangePassword(w http.ResponseWriter, r *http.Request) {
	page := viewmodel.AuthPage{
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	if err := h.renderer.RenderPage(w, r, "auth_change_password", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderChangePasswordError(w, r, "Invalid form submission")
		return
	}

	currentPassword := r.FormValue("currentPassword")
	newPassword := r.FormValue("newPassword")
	confirmPassword := r.FormValue("confirmPassword")

	if newPassword != confirmPassword {
		h.renderChangePasswordError(w, r, "New password and confirmation do not match")
		return
	}

	err := h.securityService.ChangePassword(r.Context(), user.ID, currentPassword, newPassword)
	if err != nil {
		h.renderChangePasswordError(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *AuthHandler) HandleChangePasswordComponent(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderAuthResult(w, false, "Password change failed", "Invalid form submission")
		return
	}
	if r.FormValue("newPassword") != r.FormValue("confirmPassword") {
		h.renderAuthResult(w, false, "Password change failed", "New password and confirmation do not match")
		return
	}
	if err := h.securityService.ChangePassword(r.Context(), user.ID, r.FormValue("currentPassword"), r.FormValue("newPassword")); err != nil {
		h.renderAuthResult(w, false, "Password change failed", err.Error())
		return
	}
	h.renderAuthResult(w, true, "Password changed", "Your password has been changed successfully.")
}

func (h *AuthHandler) ShowResetPassword(w http.ResponseWriter, r *http.Request) {
	page := viewmodel.AuthPage{
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	if err := h.renderer.RenderPage(w, r, "auth_reset_password", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderResetPasswordError(w, r, "Invalid form submission")
		return
	}

	email := r.FormValue("email")

	_, err := h.passwordResetSvc.RequestPasswordReset(r.Context(), email)
	if err != nil {
		h.renderResetPasswordError(w, r, err.Error())
		return
	}

	if err := h.renderer.RenderPage(w, r, "auth_reset_sent", viewmodel.AuthPage{
		CSRFToken: web.CSRFTokenFromRequest(r),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// ShowConfirmResetPassword renders the confirm-reset form, pre-populated with
// the reset token from the query string. The user arrives here by clicking the
// link in the password reset email.
func (h *AuthHandler) ShowConfirmResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	page := viewmodel.AuthPage{
		ReturnTo:  token,
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	if err := h.renderer.RenderPage(w, r, "auth_reset_confirm", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) ShowResetToken(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	query.Set("token", chi.URLParam(r, "token"))
	r.URL.RawQuery = query.Encode()
	h.ShowConfirmResetPassword(w, r)
}

// HandleConfirmResetPassword accepts the submitted new password (with the reset
// token) and delegates to PasswordResetService.ResetPassword.
func (h *AuthHandler) HandleConfirmResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderConfirmResetPasswordError(w, r, "Invalid form submission")
		return
	}

	token := r.FormValue("token")
	newPassword := r.FormValue("newPassword")

	if err := h.passwordResetSvc.ResetPassword(r.Context(), token, newPassword); err != nil {
		h.renderConfirmResetPasswordError(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/auth", http.StatusSeeOther)
}

func (h *AuthHandler) HandleConfirmResetPasswordComponent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAuthResult(w, false, "Password reset failed", "Invalid form submission")
		return
	}
	if r.FormValue("newPassword") != r.FormValue("confirmPassword") {
		h.renderAuthResult(w, false, "Password reset failed", "New password and confirmation do not match")
		return
	}
	if err := h.passwordResetSvc.ResetPassword(r.Context(), r.FormValue("token"), r.FormValue("newPassword")); err != nil {
		h.renderAuthResult(w, false, "Password reset failed", "The reset token is invalid or expired.")
		return
	}
	h.renderAuthResult(w, true, "Password reset", "Your password has been reset successfully.")
}

func isSafeRedirect(url string) bool {
	if url == "" {
		return false
	}
	if !strings.HasPrefix(url, "/") {
		return false
	}
	if strings.HasPrefix(url, "//") {
		return false
	}
	if strings.HasPrefix(url, `/\`) {
		return false
	}
	if strings.Contains(url, "://") {
		return false
	}
	return true
}

func safeReturnTo(value string) string {
	if !isSafeRedirect(value) {
		return "/"
	}
	return value
}

func (h *AuthHandler) renderAuthError(w http.ResponseWriter, r *http.Request, errMsg string) {
	// Preserve the returnTo destination across a failed login/register so the
	// post-login redirect still works after the user re-submits the form. The
	// value may come from the query string (initial GET) or the posted form
	// (the hidden input rendered on the login page).
	returnTo := r.URL.Query().Get("returnTo")
	if returnTo == "" {
		returnTo = r.FormValue("returnTo")
	}
	page := viewmodel.AuthPage{
		ReturnTo:            returnTo,
		Username:            r.FormValue("username"),
		Error:               errMsg,
		CSRFToken:           web.CSRFTokenFromRequest(r),
		GoogleLoginEnabled:  h.googleLoginEnabled,
		AppleLoginEnabled:   h.appleLoginEnabled,
		RegistrationEnabled: h.securityService.RegistrationEnabled(),
	}
	page.RegisterMode = r.URL.Path == "/auth/register" && page.RegistrationEnabled
	_ = h.renderer.RenderWithStatus(w, r, "auth_login", page, http.StatusBadRequest)
}

func (h *AuthHandler) renderTOTPError(w http.ResponseWriter, r *http.Request, errMsg string) {
	_ = h.renderer.RenderWithStatus(w, r, "auth_login", viewmodel.AuthPage{
		ReturnTo:     r.FormValue("returnTo"),
		Error:        errMsg,
		CSRFToken:    web.CSRFTokenFromRequest(r),
		TOTPRequired: true,
		PartialToken: r.FormValue("partialToken"),
	}, http.StatusUnauthorized)
}

func (h *AuthHandler) renderChangePasswordError(w http.ResponseWriter, r *http.Request, errMsg string) {
	page := viewmodel.AuthPage{
		Error:     errMsg,
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	_ = h.renderer.RenderWithStatus(w, r, "auth_change_password", page, http.StatusBadRequest)
}

func (h *AuthHandler) renderResetPasswordError(w http.ResponseWriter, r *http.Request, errMsg string) {
	page := viewmodel.AuthPage{
		Error:     errMsg,
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	_ = h.renderer.RenderWithStatus(w, r, "auth_reset_password", page, http.StatusBadRequest)
}

func (h *AuthHandler) renderConfirmResetPasswordError(w http.ResponseWriter, r *http.Request, errMsg string) {
	token := ""
	if err := r.ParseForm(); err == nil {
		token = r.FormValue("token")
	}
	page := viewmodel.AuthPage{
		Error:     errMsg,
		ReturnTo:  token,
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	_ = h.renderer.RenderWithStatus(w, r, "auth_reset_confirm", page, http.StatusBadRequest)
}
