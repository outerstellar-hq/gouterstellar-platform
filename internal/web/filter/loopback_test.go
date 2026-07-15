package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireLoopback(t *testing.T) {
	handler := RequireLoopback(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	localRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	localRequest.RemoteAddr = "127.0.0.1:1234"
	localResponse := httptest.NewRecorder()
	handler.ServeHTTP(localResponse, localRequest)
	assert.Equal(t, http.StatusNoContent, localResponse.Code)

	remoteRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	remoteRequest.RemoteAddr = "203.0.113.10:1234"
	remoteRequest.Header.Set("X-Forwarded-For", "127.0.0.1")
	remoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(remoteResponse, remoteRequest)
	assert.Equal(t, http.StatusNotFound, remoteResponse.Code)
}
