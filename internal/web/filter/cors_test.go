package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORSExposesSessionHeadersAndAllowsCorrelationID(t *testing.T) {
	handler := CORS("*")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil))
	assert.Contains(t, response.Header().Get("Access-Control-Expose-Headers"), RequestIDHeader)
	assert.Contains(t, response.Header().Get("Access-Control-Expose-Headers"), SessionExpiredHeader)

	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodOptions, "/api/v1/sync", nil))
	assert.Equal(t, http.StatusNoContent, preflight.Code)
	assert.Contains(t, preflight.Header().Get("Access-Control-Allow-Headers"), RequestIDHeader)
}
