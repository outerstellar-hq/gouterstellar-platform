// Package web provides application-neutral HTTP security conventions.
package web

import (
	"html/template"
	"net/http"
	"time"

	csrf "filippo.io/csrf/gorilla"
)

// CSRFConfig is retained for API compatibility with the previous
// token-based middleware. Protection now uses browser Fetch metadata
// (Sec-Fetch-Site / Origin), so tokens, cookies, and keys are no longer
// part of the mechanism: AuthKey, CookieName, MaxAge, and
// AllowInsecureCookies are accepted and ignored. FieldName still controls
// the name attribute of the compatibility hidden input rendered by
// [CSRFField], and ErrorHandler still receives denied requests.
type CSRFConfig struct {
	AuthKey              []byte
	CookieName           string
	FieldName            string
	MaxAge               time.Duration
	AllowInsecureCookies bool
	ErrorHandler         http.Handler
}

// NewCSRF returns CSRF middleware for cookie-authenticated browser routes,
// based on filippo.io/csrf (the maintained drop-in replacement for
// github.com/gorilla/csrf, replacing it per GHSA-82ff-hg59-8x73 /
// GO-2025-3884, which affects gorilla/csrf through v1.7.3 with no fixed
// release planned).
//
// Safe methods (GET, HEAD, OPTIONS) and same-origin or non-browser requests
// are allowed; cross-origin unsafe requests are rejected. Tokens and
// cookies are not used: [CSRFToken] returns an ignored random value and
// [CSRFField] renders an ignored hidden input, so existing forms keep
// working unchanged. Bearer-token APIs should use a separate router without
// CSRF middleware.
//
// NewCSRF no longer fails on weak keys or unusual MaxAge values; the error
// result is retained for signature compatibility and is always nil.
func NewCSRF(config CSRFConfig) (func(http.Handler) http.Handler, error) {
	options := []csrf.Option{}
	if config.FieldName != "" {
		options = append(options, csrf.FieldName(config.FieldName))
	} else {
		options = append(options, csrf.FieldName("csrf_token"))
	}
	if config.ErrorHandler != nil {
		options = append(options, csrf.ErrorHandler(config.ErrorHandler))
	}
	return csrf.Protect(nil, options...), nil
}

// CSRFToken returns a masked per-request token for JSON or custom templates.
// The value is not validated by the middleware and is provided for
// compatibility with token-based clients.
func CSRFToken(request *http.Request) string {
	return csrf.Token(request)
}

// CSRFField returns a safe hidden input for html/template forms. The input
// is ignored by the middleware and is provided so existing forms render
// unchanged.
func CSRFField(request *http.Request) template.HTML {
	return csrf.TemplateField(request)
}
