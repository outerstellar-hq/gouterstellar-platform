package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestETagPreservesResponseAndReturnsNotModified(t *testing.T) {
	handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Content-Length", "21")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("body { color: navy; }"))
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/site.css", nil))
	require.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "body { color: navy; }", first.Body.String())
	assert.Equal(t, "text/css; charset=utf-8", first.Header().Get("Content-Type"))
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)

	second := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/site.css", nil)
	request.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(second, request)
	assert.Equal(t, http.StatusNotModified, second.Code)
	assert.Empty(t, second.Body.String())
	assert.Empty(t, second.Header().Get("Content-Length"))
	assert.Equal(t, etag, second.Header().Get("ETag"))
}

func TestETagLeavesNonSuccessfulAndNonGETResponsesAlone(t *testing.T) {
	notFound := ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	recorder := httptest.NewRecorder()
	notFound.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing.css", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Empty(t, recorder.Header().Get("ETag"))
	assert.Contains(t, recorder.Body.String(), "missing")

	post := ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder = httptest.NewRecorder()
	post.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/asset", nil))
	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Empty(t, recorder.Header().Get("ETag"))
}
