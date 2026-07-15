package filter

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/web"
)

const csrfCookieName = "outerstellar_csrf"

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func CSRF(enabled, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := csrfToken(w, r, secure)
			if err != nil {
				http.Error(w, "Unable to secure request", http.StatusInternalServerError)
				return
			}
			r = web.WithCSRFToken(r, token)

			if enabled && !strings.HasPrefix(r.URL.Path, "/api/") && isUnsafeMethod(r.Method) && !validCSRFRequest(r, token) {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func csrfToken(w http.ResponseWriter, r *http.Request, secure bool) (string, error) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && len(cookie.Value) == 64 {
		return cookie.Value, nil
	}
	token, err := generateCSRFToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- attributes are explicit; Secure is enabled outside the local development profile
		Name: csrfCookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: secure, SameSite: http.SameSiteStrictMode,
	})
	return token, nil
}

func validCSRFRequest(r *http.Request, token string) bool {
	submitted := r.Header.Get("X-CSRF-Token")
	if submitted == "" {
		submitted = r.FormValue("csrf_token")
	}
	if submitted != "" {
		return subtle.ConstantTimeCompare([]byte(submitted), []byte(token)) == 1
	}
	return sameOrigin(r.Header.Get("Origin"), r.Host) || sameOrigin(r.Header.Get("Referer"), r.Host)
}

func sameOrigin(rawURL, host string) bool {
	if rawURL == "" {
		return false
	}
	parsed, err := url.Parse(rawURL)
	return err == nil && strings.EqualFold(parsed.Host, host)
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}
