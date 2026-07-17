package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

func newErrorHandlerForTest(t *testing.T) *ErrorHandler {
	t.Helper()
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	require.NoError(t, err)
	return NewErrorHandler(renderer, "test")
}

func requestWithID(method, path, requestID string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(request.Context(), web.ContextKey("requestId"), requestID)
	return request.WithContext(ctx)
}

func TestNotFoundReturnsRequestAwareJSONForAPIPaths(t *testing.T) {
	recorder := httptest.NewRecorder()
	newErrorHandlerForTest(t).NotFound(recorder, requestWithID(http.MethodGet, "/api/v1/missing", "request-404"))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var payload map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "not_found", payload["error"])
	assert.Equal(t, "request-404", payload["requestId"])
	assert.NotEmpty(t, payload["message"])
}

func TestInternalErrorNegotiatesAPIAndHTMXResponses(t *testing.T) {
	handler := newErrorHandlerForTest(t)

	api := httptest.NewRecorder()
	handler.InternalError(api, requestWithID(http.MethodGet, "/api/v1/failure", "request-500"), errors.New("database password leaked"))
	require.Equal(t, http.StatusInternalServerError, api.Code)
	assert.Equal(t, "application/json", api.Header().Get("Content-Type"))
	assert.Contains(t, api.Body.String(), "request-500")
	assert.NotContains(t, api.Body.String(), "database password leaked")

	htmxRequest := requestWithID(http.MethodPost, "/messages/action", "request-htmx")
	htmxRequest.Header.Set("HX-Request", "true")
	htmx := httptest.NewRecorder()
	handler.InternalError(htmx, htmxRequest, errors.New("action failed internally"))
	require.Equal(t, http.StatusInternalServerError, htmx.Code)
	assert.Equal(t, "text/plain; charset=utf-8", htmx.Header().Get("Content-Type"))
	assert.Equal(t, "Action failed", htmx.Body.String())
}

func TestNotFoundKeepsThemedHTMLForBrowserPaths(t *testing.T) {
	recorder := httptest.NewRecorder()
	newErrorHandlerForTest(t).NotFound(recorder, requestWithID(http.MethodGet, "/missing-page", "request-html"))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), "request-html")
	assert.Contains(t, recorder.Body.String(), "<!DOCTYPE html>")
}
