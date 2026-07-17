package filter

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

// SecurityHeaders sets standard security headers and injects a per-request
// CSP nonce into the Content-Security-Policy header.
//
// If cspPolicy is empty, a safe default policy is used. If cspPolicy contains
// the literal "{nonce}", it is replaced with the generated nonce; otherwise
// the policy is applied as-is.
func SecurityHeaders(cspPolicy string, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := generateNonce()
			r = web.WithCSPNonce(r, nonce)

			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			if !isAPIPath(r.URL.Path) {
				csp := cspPolicy
				if csp == "" {
					csp = "default-src 'self'; script-src 'self' 'nonce-" + nonce + "'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'"
				} else {
					csp = strings.ReplaceAll(csp, "{nonce}", "'nonce-"+nonce+"'")
				}
				w.Header().Set("Content-Security-Policy", csp)
			}

			if secure {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// generateNonce returns a URL-safe base64-encoded 16-byte random nonce suitable for use
// in a CSP 'nonce-' source. Errors from rand.Read are ignored because a
// depleted entropy pool would already be fatal to request handling; the
// resulting (possibly zero) nonce still conforms to the CSP grammar.
func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
