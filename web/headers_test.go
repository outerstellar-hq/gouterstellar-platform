package web

import (
	"io"
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

func TestNoStoreMarksSensitiveResponses(t *testing.T) {
	handler := NoStore(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusNoContent || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d cache-control=%q", response.Code, response.Header().Get("Cache-Control"))
	}
}

func TestMaxBodyBytesBoundsHandlerReads(t *testing.T) {
	handler := MaxBodyBytes(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	for name, test := range map[string]struct {
		body       string
		wantStatus int
	}{
		"at limit":   {body: "1234", wantStatus: http.StatusNoContent},
		"over limit": {body: "12345", wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body)))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
