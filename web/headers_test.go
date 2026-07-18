package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersInjectsNonce(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{HSTS: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if CSPNonce(r.Context()) == "" {
			t.Fatal("nonce missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://example.test", nil))
	policy := response.Header().Get("Content-Security-Policy")
	if strings.Contains(policy, "{nonce}") || !strings.Contains(policy, "nonce-") {
		t.Fatalf("policy = %q", policy)
	}
	for _, header := range []string{"Permissions-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options", "Cross-Origin-Opener-Policy", "Strict-Transport-Security"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("missing %s", header)
		}
	}
}
