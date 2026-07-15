package filter

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/web"
)

const csrfCookieName = "oss_csrf"

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CSRF applies a cookie-backed double-submit token to browser requests. The
// HttpOnly cookie keeps the stable token server-readable while templates and
// the request context expose only the value needed by the current page.
func CSRF(enabled, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// API clients authenticate with a Bearer token (Authorization:
			// Bearer ...) rather than the session cookie + CSRF token that
			// browser forms use. A request carrying a Bearer header is therefore
			// exempt from CSRF: it is not vulnerable to cookie-based CSRF and
			// has no access to the CSRF token embedded in the rendered form.
			// This replaces the former URL-prefix ("/api/") sniff, which both
			// exempted legitimate API traffic and silently bypassed CSRF for any
			// path that happened to start with /api/.
			if hasBearerAuth(r) {
				next.ServeHTTP(w, r)
				return
			}

			token, hasToken := csrfTokenFromCookie(r)
			if !hasToken {
				var err error
				token, err = generateCSRFToken()
				if err != nil {
					slog.Error("Failed to initialize CSRF protection", "error", err)
					http.Error(w, "Unable to initialize request protection", http.StatusInternalServerError)
					return
				}
				http.SetCookie(w, &http.Cookie{ // #nosec G124 -- all security attributes are explicitly configured
					Name:     csrfCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					Secure:   secure,
					SameSite: http.SameSiteStrictMode,
				})
			}
			r = web.WithCSRFToken(r, token)

			if enabled && isUnsafeMethod(r.Method) {
				submitted := r.Header.Get("X-CSRF-Token")
				if submitted == "" {
					submitted = r.FormValue("csrf_token")
				}
				if !hasToken || subtle.ConstantTimeCompare([]byte(submitted), []byte(token)) != 1 {
					http.Error(w, "Invalid CSRF token", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func csrfTokenFromCookie(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || len(cookie.Value) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(cookie.Value); err != nil {
		return "", false
	}
	return cookie.Value, true
}

// hasBearerAuth reports whether the request carries an Authorization header
// whose scheme is Bearer (case-insensitive per RFC 7235). Trailing whitespace
// around the scheme is tolerated.
func hasBearerAuth(r *http.Request) bool {
	v := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(v) <= len(prefix) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(v[:len(prefix)]), "bearer")
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}
