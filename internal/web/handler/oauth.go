package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type OAuthHandler struct {
	securityService *service.SecurityService
	oauthService    *security.OAuthService
	sessionSecure   bool
	appleProvider   security.OAuthProvider
	appBaseURL      string
}

func NewOAuthHandler(
	secSvc *service.SecurityService,
	oauthSvc *security.OAuthService,
	sessionSecure bool,
	appleProvider security.OAuthProvider,
	appBaseURL string,
) *OAuthHandler {
	return &OAuthHandler{
		securityService: secSvc,
		oauthService:    oauthSvc,
		sessionSecure:   sessionSecure,
		appleProvider:   appleProvider,
		appBaseURL:      appBaseURL,
	}
}

func (h *OAuthHandler) RegisterRoutes(r chi.Router) {
	r.Get("/auth/oauth/{provider}", h.Redirect)
	r.Get("/auth/oauth/{provider}/callback", h.Callback)
	r.Post("/auth/oauth/{provider}/callback", h.CallbackPost)
}

func (h *OAuthHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider := h.resolveProvider(providerName)
	if provider == nil {
		writeError(w, http.StatusBadRequest, "Unknown OAuth provider")
		return
	}

	state := generateOAuthState()
	redirectURI := h.appBaseURL + "/auth/oauth/" + providerName + "/callback"

	authURL := provider.AuthorizationURL(state, redirectURI)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	h.handleCallback(w, r)
}

func (h *OAuthHandler) CallbackPost(w http.ResponseWriter, r *http.Request) {
	h.handleCallback(w, r)
}

func (h *OAuthHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider := h.resolveProvider(providerName)
	if provider == nil {
		writeError(w, http.StatusBadRequest, "Unknown OAuth provider")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		code = r.FormValue("code")
	}
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing authorization code")
		return
	}

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing state cookie")
		return
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam == "" {
		stateParam = r.FormValue("state")
	}
	if stateParam != stateCookie.Value {
		writeError(w, http.StatusBadRequest, "Invalid OAuth state")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionSecure,
		MaxAge:   -1,
	})

	redirectURI := h.appBaseURL + "/auth/oauth/" + providerName + "/callback"
	userInfo, err := provider.ExchangeCode(code, stateParam, redirectURI)
	if err != nil {
		slog.Error("OAuth code exchange failed", "provider", providerName, "error", err)
		writeError(w, http.StatusInternalServerError, "OAuth authentication failed")
		return
	}

	user, err := h.oauthService.FindOrCreateOAuthUser(r.Context(), providerName, userInfo.Subject, userInfo.Email)
	if err != nil {
		slog.Error("OAuth user creation failed", "provider", providerName, "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to create OAuth user")
		return
	}

	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *OAuthHandler) resolveProvider(name string) security.OAuthProvider {
	switch name {
	case "apple":
		return h.appleProvider
	default:
		return nil
	}
}

func generateOAuthState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
