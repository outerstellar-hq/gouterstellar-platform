package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The middleware uses browser Fetch metadata (Sec-Fetch-Site / Origin):
// same-origin and non-browser requests pass; cross-origin unsafe requests
// are denied. Tokens and cookies are compatibility shims only.
func TestCSRFAllowsSameOriginAndDeniesCrossOrigin(t *testing.T) {
	middleware, err := NewCSRF(CSRFConfig{AuthKey: bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := CSRFToken(r)
		if token == "" {
			t.Fatal("compatibility token missing")
		}
		if field := string(CSRFField(r)); !strings.Contains(field, `name="csrf_token"`) {
			t.Fatalf("CSRF field = %q", field)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// Safe methods pass and issue a compatibility token without cookies.
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "https://example.test/form", nil))
	if getResponse.Code != http.StatusNoContent {
		t.Fatalf("GET status = %d", getResponse.Code)
	}
	if cookies := getResponse.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("expected no CSRF cookie, got %#v", cookies)
	}

	// Same-origin browser POST: allowed without any token.
	sameOrigin := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader("a=1"))
	sameOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sameOrigin.Header.Set("Sec-Fetch-Site", "same-origin")
	sameOriginResponse := httptest.NewRecorder()
	handler.ServeHTTP(sameOriginResponse, sameOrigin)
	if sameOriginResponse.Code != http.StatusNoContent {
		t.Fatalf("same-origin status = %d", sameOriginResponse.Code)
	}

	// Non-browser requests (no Fetch metadata or Origin) are allowed.
	plain := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader("a=1"))
	plain.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	plainResponse := httptest.NewRecorder()
	handler.ServeHTTP(plainResponse, plain)
	if plainResponse.Code != http.StatusNoContent {
		t.Fatalf("non-browser status = %d", plainResponse.Code)
	}

	// Cross-site browser POST: denied.
	crossSite := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader("a=1"))
	crossSite.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSiteResponse := httptest.NewRecorder()
	handler.ServeHTTP(crossSiteResponse, crossSite)
	if crossSiteResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-site status = %d", crossSiteResponse.Code)
	}

	// Origin header mismatch on a browser-less POST: denied when Origin
	// signals another origin.
	crossOrigin := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader("a=1"))
	crossOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossOrigin.Header.Set("Origin", "https://untrusted.example")
	crossOriginResponse := httptest.NewRecorder()
	handler.ServeHTTP(crossOriginResponse, crossOrigin)
	if crossOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", crossOriginResponse.Code)
	}
}

func TestCSRFConfigBackwardCompatible(t *testing.T) {
	// Legacy configuration (weak keys, unusual MaxAge, cookie settings) is
	// accepted: those fields no longer participate in the mechanism.
	if _, err := NewCSRF(CSRFConfig{}); err != nil {
		t.Fatalf("empty config rejected: %v", err)
	}
	if _, err := NewCSRF(CSRFConfig{AuthKey: []byte("weak")}); err != nil {
		t.Fatalf("weak key rejected: %v", err)
	}
}
