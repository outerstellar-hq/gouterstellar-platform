// Package web provides application-neutral HTTP security conventions.
package web

import (
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/csrf"
)

const maxCSRFMaxAge = time.Duration(1<<31-1) * time.Second

// CSRFConfig controls cookie-backed masked CSRF tokens. Secure, HttpOnly,
// SameSite=Strict cookies and whole-application path coverage are enforced.
type CSRFConfig struct {
	AuthKey              []byte
	CookieName           string
	FieldName            string
	MaxAge               time.Duration
	AllowInsecureCookies bool
	ErrorHandler         http.Handler
}

// NewCSRF returns Gorilla CSRF middleware with platform security defaults for
// cookie-authenticated browser routes. Bearer-token APIs should use a separate
// router without cookie-backed CSRF middleware.
func NewCSRF(config CSRFConfig) (func(http.Handler) http.Handler, error) {
	if len(config.AuthKey) < 32 {
		return nil, errors.New("CSRF authentication key must contain at least 32 bytes")
	}
	if config.CookieName == "" {
		config.CookieName = "outerstellar_csrf"
	}
	if config.FieldName == "" {
		config.FieldName = "csrf_token"
	}
	if config.MaxAge < 0 {
		return nil, errors.New("CSRF max age cannot be negative")
	}
	if config.MaxAge > 0 && (config.MaxAge < time.Second || config.MaxAge%time.Second != 0 || config.MaxAge > maxCSRFMaxAge) {
		return nil, errors.New("positive CSRF max age must use whole-second precision and fit in 32-bit seconds")
	}
	maxAge := 12 * time.Hour
	if config.MaxAge > 0 {
		maxAge = config.MaxAge
	}

	options := []csrf.Option{
		csrf.CookieName(config.CookieName),
		csrf.FieldName(config.FieldName),
		csrf.Path("/"),
		csrf.HttpOnly(true),
		csrf.Secure(!config.AllowInsecureCookies),
		csrf.SameSite(csrf.SameSiteStrictMode),
		csrf.MaxAge(int(maxAge.Seconds())),
	}
	if config.ErrorHandler != nil {
		options = append(options, csrf.ErrorHandler(config.ErrorHandler))
	}
	return csrf.Protect(append([]byte(nil), config.AuthKey...), options...), nil
}

// CSRFToken returns the masked per-request token for JSON or custom templates.
func CSRFToken(request *http.Request) string {
	return csrf.Token(request)
}

// CSRFField returns a safe hidden input for html/template forms.
func CSRFField(request *http.Request) template.HTML {
	return csrf.TemplateField(request)
}
