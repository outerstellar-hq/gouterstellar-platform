package filter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/web"
)

func csrfTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, web.CSRFTokenFromRequest(r))
	})
}

func TestCSRFAcceptsTokenIssuedWithRenderedForm(t *testing.T) {
	handler := CSRF(true, false)(csrfTestHandler())
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/settings", nil))

	response := getRecorder.Result()
	require.Len(t, response.Cookies(), 1)
	cookie := response.Cookies()[0]
	token := getRecorder.Body.String()
	require.Equal(t, token, cookie.Value)

	form := url.Values{"csrf_token": {token}}
	post := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.AddCookie(cookie)
	postRecorder := httptest.NewRecorder()
	handler.ServeHTTP(postRecorder, post)

	assert.Equal(t, http.StatusOK, postRecorder.Code)
	assert.Equal(t, token, postRecorder.Body.String())
}

func TestCSRFRejectsMissingOrMismatchedToken(t *testing.T) {
	handler := CSRF(true, false)(csrfTestHandler())

	missingRecorder := httptest.NewRecorder()
	handler.ServeHTTP(missingRecorder, httptest.NewRequest(http.MethodPost, "/settings", nil))
	assert.Equal(t, http.StatusForbidden, missingRecorder.Code)
	assert.NotEmpty(t, missingRecorder.Result().Cookies(), "a replacement token should be issued for the next rendered form")

	mismatch := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader("csrf_token=wrong"))
	mismatch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatch.AddCookie(&http.Cookie{Name: csrfCookieName, Value: strings.Repeat("a", 64)})
	mismatchRecorder := httptest.NewRecorder()
	handler.ServeHTTP(mismatchRecorder, mismatch)
	assert.Equal(t, http.StatusForbidden, mismatchRecorder.Code)
}

func TestCSRFBearerRequestsRemainCookieFree(t *testing.T) {
	handler := CSRF(true, true)(csrfTestHandler())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sync", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Result().Cookies())
	assert.Empty(t, recorder.Body.String())
}

func TestCSRFDisabledStillProvidesTemplateToken(t *testing.T) {
	handler := CSRF(false, true)(csrfTestHandler())
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/settings", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, recorder.Result().Cookies(), 1)
	assert.True(t, recorder.Result().Cookies()[0].Secure)
	assert.Equal(t, recorder.Result().Cookies()[0].Value, recorder.Body.String())
}
