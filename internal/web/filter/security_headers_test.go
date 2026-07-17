package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

func TestSecurityHeadersApplyCSPOnlyToBrowserRoutes(t *testing.T) {
	handler := SecurityHeaders("default-src 'self'; script-src 'self' {nonce}", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	browser := httptest.NewRecorder()
	handler.ServeHTTP(browser, httptest.NewRequest(http.MethodGet, "/auth", nil))
	require.Equal(t, http.StatusNoContent, browser.Code)
	assert.Contains(t, browser.Header().Get("Content-Security-Policy"), "'nonce-")
	assert.Equal(t, "nosniff", browser.Header().Get("X-Content-Type-Options"))

	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil))
	require.Equal(t, http.StatusNoContent, api.Code)
	assert.Empty(t, api.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "DENY", api.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, api.Header().Get("Referrer-Policy"))
}

func TestSecurityHeadersSharesNonceWithRenderer(t *testing.T) {
	var requestNonce string
	handler := SecurityHeaders("script-src 'self' {nonce}", false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNonce = web.CSPNonceFromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth", nil))

	require.NotEmpty(t, requestNonce)
	assert.Contains(t, recorder.Header().Get("Content-Security-Policy"), "'nonce-"+requestNonce+"'")
}
