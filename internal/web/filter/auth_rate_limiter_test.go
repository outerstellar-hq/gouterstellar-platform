package filter

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiterMatchesJavaBoundaries(t *testing.T) {
	t.Run("limits only authentication paths", func(t *testing.T) {
		handler := AuthRateLimiter(10, time.Minute, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		for range 15 {
			response := serveRateLimited(handler, http.MethodGet, "/health", "203.0.113.1:1234", "", "")
			require.Equal(t, http.StatusNoContent, response.Code)
		}
		for i := range 10 {
			body := `{"username":"user-` + string(rune('a'+i)) + `"}`
			response := serveRateLimited(handler, http.MethodPost, "/api/v1/auth/login", "203.0.113.1:1234", "application/json", body)
			require.Equal(t, http.StatusNoContent, response.Code)
		}
		blocked := serveRateLimited(handler, http.MethodPost, "/api/v1/auth/login", "203.0.113.1:1234", "application/json", `{"username":"last"}`)
		assert.Equal(t, http.StatusTooManyRequests, blocked.Code)
	})

	t.Run("reset endpoints use the five request window", func(t *testing.T) {
		handler := AuthRateLimiter(10, time.Minute, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		for range 5 {
			response := serveRateLimited(handler, http.MethodPost, "/api/v1/auth/reset-confirm", "203.0.113.2:1234", "application/json", `{}`)
			require.Equal(t, http.StatusNoContent, response.Code)
		}
		blocked := serveRateLimited(handler, http.MethodPost, "/api/v1/auth/reset-confirm", "203.0.113.2:1234", "application/json", `{}`)
		assert.Equal(t, http.StatusTooManyRequests, blocked.Code)
	})
}

func TestAuthRateLimiterTrustsOnlyConfiguredProxies(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	trustedHandler := AuthRateLimiter(2, time.Minute, []string{"10.0.0.1"})(next)
	for range 2 {
		response := serveForwardedRequest(trustedHandler, "10.0.0.1:4321", "1.2.3.4")
		require.Equal(t, http.StatusNoContent, response.Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, serveForwardedRequest(trustedHandler, "10.0.0.1:4321", "1.2.3.4").Code)
	assert.Equal(t, http.StatusNoContent, serveForwardedRequest(trustedHandler, "10.0.0.1:4321", "5.6.7.8").Code)

	untrustedHandler := AuthRateLimiter(2, time.Minute, []string{"10.0.0.1"})(next)
	assert.Equal(t, http.StatusNoContent, serveForwardedRequest(untrustedHandler, "198.51.100.2:4321", "1.1.1.1").Code)
	assert.Equal(t, http.StatusNoContent, serveForwardedRequest(untrustedHandler, "198.51.100.2:4321", "2.2.2.2").Code)
	assert.Equal(t, http.StatusTooManyRequests, serveForwardedRequest(untrustedHandler, "198.51.100.2:4321", "3.3.3.3").Code)
}

func TestAuthRateLimiterCapsAccountsAcrossClientIPsAndPreservesBodies(t *testing.T) {
	var observedBody string
	handler := AuthRateLimiter(100, time.Minute, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		observedBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))

	jsonBody := `{"username":"Target-User","password":"wrong"}`
	for i := 1; i <= 20; i++ {
		peer := fmt.Sprintf("10.99.0.%d:1234", i)
		response := serveRateLimited(handler, http.MethodPost, "/api/v1/auth/login", peer, "application/json", jsonBody)
		require.Equal(t, http.StatusNoContent, response.Code)
		assert.Equal(t, jsonBody, observedBody)
	}
	blocked := serveRateLimited(handler, http.MethodPost, "/api/v1/auth/login", "10.99.1.1:1234", "application/json", jsonBody)
	assert.Equal(t, http.StatusTooManyRequests, blocked.Code)

	formHandler := AuthRateLimiter(100, time.Minute, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	formBody := "mode=sign-in&email=target%40example.com&password=wrong"
	for i := 1; i <= 20; i++ {
		peer := fmt.Sprintf("10.77.0.%d:1234", i)
		require.Equal(t, http.StatusNoContent, serveRateLimited(formHandler, http.MethodPost, "/auth/components/result", peer, "application/x-www-form-urlencoded", formBody).Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, serveRateLimited(formHandler, http.MethodPost, "/auth/components/result", "10.77.1.1:1234", "application/x-www-form-urlencoded", formBody).Code)
}

func serveForwardedRequest(handler http.Handler, remoteAddr, forwardedFor string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"unique"}`))
	request.RemoteAddr = remoteAddr
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", forwardedFor)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func serveRateLimited(handler http.Handler, method, path, remoteAddr, contentType, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = remoteAddr
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
