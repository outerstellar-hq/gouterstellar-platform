package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type AuthHandler struct {
	securityService  *service.SecurityService
	passwordResetSvc *service.PasswordResetService
	renderer         *web.Renderer
	sessionSecure    bool
	analytics        service.AnalyticsService
}

func NewAuthHandler(
	secSvc *service.SecurityService,
	passwordResetSvc *service.PasswordResetService,
	renderer *web.Renderer,
	sessionSecure bool,
	analytics service.AnalyticsService,
) *AuthHandler {
	return &AuthHandler{
		securityService:  secSvc,
		passwordResetSvc: passwordResetSvc,
		renderer:         renderer,
		sessionSecure:    sessionSecure,
		analytics:        analytics,
	}
}

func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Get("/auth", h.ShowLogin)
	r.Post("/auth/login", h.HandleLogin)
	r.Post("/auth/register", h.HandleRegister)
	r.Post("/auth/logout", h.HandleLogout)
	r.Get("/auth/change-password", h.ShowChangePassword)
	r.Post("/auth/change-password", h.HandleChangePassword)
	r.Get("/auth/reset", h.ShowResetPassword)
	r.Post("/auth/reset", h.HandleResetPassword)
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("returnTo")
	page := viewmodel.AuthPage{
		ReturnTo:  returnTo,
		CSRFToken: web.CSRFTokenFromRequest(r),
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

	user, err := h.securityService.Authenticate(r.Context(), username, password)
	if err != nil {
		h.renderAuthError(w, r, "Invalid username or password")
		return
	}

	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.renderAuthError(w, r, "Failed to create session")
		return
	}

	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))

	h.analytics.Track(r.Context(), "user_login", map[string]interface{}{
		"username": user.Username,
	})

	returnTo := r.FormValue("returnTo")
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

	user, err := h.securityService.Register(r.Context(), username, password)
	if err != nil {
		h.renderAuthError(w, r, err.Error())
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

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
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

	err := h.securityService.ChangePassword(r.Context(), user.ID, currentPassword, newPassword)
	if err != nil {
		h.renderChangePasswordError(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
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

	_ = h.renderer.RenderPage(w, r, "auth_reset_sent", viewmodel.AuthPage{
		CSRFToken: web.CSRFTokenFromRequest(r),
	})
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
	if strings.Contains(url, "://") {
		return false
	}
	return true
}

func (h *AuthHandler) renderAuthError(w http.ResponseWriter, r *http.Request, errMsg string) {
	page := viewmodel.AuthPage{
		Error:     errMsg,
		CSRFToken: web.CSRFTokenFromRequest(r),
	}
	_ = h.renderer.RenderWithStatus(w, r, "auth_login", page, http.StatusBadRequest)
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
