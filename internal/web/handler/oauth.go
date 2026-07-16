package handler

import (
	"crypto/rand"
	"encoding/hex"
	"html"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/security"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type OAuthHandler struct {
	securityService *service.SecurityService
	oauthService    *security.OAuthService
	sessionSecure   bool
	appleProvider   security.OAuthProvider
	googleProvider  security.OAuthProvider
	appBaseURL      string
}

func NewOAuthHandler(
	secSvc *service.SecurityService,
	oauthSvc *security.OAuthService,
	sessionSecure bool,
	appleProvider security.OAuthProvider,
	googleProvider security.OAuthProvider,
	appBaseURL string,
) *OAuthHandler {
	return &OAuthHandler{
		securityService: secSvc,
		oauthService:    oauthSvc,
		sessionSecure:   sessionSecure,
		appleProvider:   appleProvider,
		googleProvider:  googleProvider,
		appBaseURL:      appBaseURL,
	}
}

// ContributeRoutes registers the OAuth routes (public).
func (h *OAuthHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}", "OAuth redirect", http.HandlerFunc(h.Redirect))
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}/callback", "OAuth callback", http.HandlerFunc(h.Callback))
	ctx.Routes.Public(http.MethodPost, "/auth/oauth/{provider}/callback", "OAuth callback POST", http.HandlerFunc(h.CallbackPost))
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}/not-configured", "OAuth provider not configured", http.HandlerFunc(h.NotConfigured))
	return nil
}

func (h *OAuthHandler) NotConfigured(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	if h.resolveProvider(providerName) == nil {
		writeError(w, http.StatusBadRequest, "Unknown OAuth provider")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("<h2>Sign in with " + html.EscapeString(providerName) + " is not yet configured.</h2>" +
		"<p>The provider is not available in this release. Please contact the administrator.</p>" +
		"<a href='/auth'>Back to sign in</a>"))
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
	if authURL == "" {
		writeError(w, http.StatusNotImplemented, "OAuth provider not configured")
		return
	}

	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- attributes set; Secure is parameterized per-environment
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	http.Redirect(w, r, authURL, http.StatusFound)
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

	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- attributes set; Secure is parameterized per-environment
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionSecure,
		SameSite: http.SameSiteLaxMode,
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
	case "google":
		return h.googleProvider
	default:
		return nil
	}
}

func generateOAuthState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
