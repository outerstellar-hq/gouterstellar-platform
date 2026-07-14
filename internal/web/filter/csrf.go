package filter

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/web"
)

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Error("Failed to generate CSRF token", "error", err)
		return ""
	}
	return hex.EncodeToString(b)
}

func CSRF(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := generateCSRFToken()
			r = web.WithCSRFToken(r, token)

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

			if enabled && isUnsafeMethod(r.Method) {
				submitted := r.Header.Get("X-CSRF-Token")
				if submitted == "" {
					submitted = r.FormValue("csrf_token")
				}
				if submitted != token {
					http.Error(w, "Invalid CSRF token", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
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
