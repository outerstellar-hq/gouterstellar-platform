package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorHandlerUsesThemedRendererForPanics(t *testing.T) {
	var recovered error
	middleware := ErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
		recovered = err
		http.Error(w, "themed server error", http.StatusInternalServerError)
	})
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	require.Error(t, recovered)
	assert.Contains(t, recovered.Error(), "boom")
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "themed server error")
}
