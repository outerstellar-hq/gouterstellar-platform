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
		writeError(w, http.StatusNotFound, "Unknown OAuth provider")
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
		writeError(w, http.StatusNotFound, "Unknown OAuth provider")
		return
	}

	state, err := generateOAuthState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start OAuth authentication")
		return
	}
	redirectURI := h.appBaseURL + "/auth/oauth/" + providerName + "/callback"

	authURL := provider.AuthorizationURL(state, redirectURI)
	if authURL == "" {
		http.Redirect(w, r, "/auth/oauth/"+providerName+"/not-configured", http.StatusFound) // #nosec G710 -- resolveProvider restricts providerName to the apple/google allowlist.
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
		writeError(w, http.StatusNotFound, "Unknown OAuth provider")
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.oauthError(w, r, providerName, "invalid callback form", err)
			return
		}
	}
	if providerError := r.FormValue("error"); providerError != "" {
		h.oauthError(w, r, providerName, "provider rejected authentication", nil)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		code = r.FormValue("code")
	}
	if code == "" {
		h.oauthError(w, r, providerName, "missing authorization code", nil)
		return
	}

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		h.oauthError(w, r, providerName, "missing state cookie", nil)
		return
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam == "" {
		stateParam = r.FormValue("state")
	}
	if stateParam != stateCookie.Value {
		h.oauthError(w, r, providerName, "invalid state", nil)
		return
	}

	h.clearStateCookie(w)

	redirectURI := h.appBaseURL + "/auth/oauth/" + providerName + "/callback"
	userInfo, err := provider.ExchangeCode(code, stateParam, redirectURI)
	if err != nil {
		h.oauthError(w, r, providerName, "code exchange failed", err)
		return
	}

	user, err := h.oauthService.FindOrCreateOAuthUser(r.Context(), providerName, userInfo.Subject, userInfo.Email)
	if err != nil {
		h.oauthError(w, r, providerName, "user creation failed", err)
		return
	}

	token, err := h.securityService.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.oauthError(w, r, providerName, "session creation failed", err)
		return
	}

	http.SetCookie(w, web.CreateSessionCookie(token, h.sessionSecure))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *OAuthHandler) oauthError(w http.ResponseWriter, r *http.Request, providerName, message string, err error) {
	if err == nil {
		slog.Warn("OAuth callback rejected", "provider", providerName, "reason", message)
	} else {
		slog.Error("OAuth callback failed", "provider", providerName, "reason", message, "error", err)
	}
	h.clearStateCookie(w)
	http.Redirect(w, r, "/auth?oauth_error=true", http.StatusFound)
}

func (h *OAuthHandler) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- attributes set; Secure is parameterized per-environment
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
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

func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
