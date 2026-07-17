package filter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackingBody struct {
	read bool
}

func (b *trackingBody) Read([]byte) (int, error) {
	b.read = true
	return 0, io.EOF
}

func (*trackingBody) Close() error { return nil }

func TestMaxBodySizePassesUnknownAndAllowedLengths(t *testing.T) {
	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/auth", nil),
		httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(strings.Repeat("x", 100))),
	}
	requests[0].ContentLength = -1
	requests[0].Header.Set("Content-Length", "not-a-number")
	requests[1].ContentLength = 100

	for _, request := range requests {
		reached := false
		handler := MaxBodySize(100)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		}))
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.True(t, reached)
	}
}

func TestMaxBodySizeRejectsDeclaredOversizeWithoutReadingBody(t *testing.T) {
	body := &trackingBody{}
	request := httptest.NewRequest(http.MethodPost, "/auth", nil)
	request.Body = body
	request.ContentLength = 500 * 1024 * 1024
	reached := false
	handler := MaxBodySize(2 * 1024 * 1024)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), "exceeds the limit")
	assert.False(t, reached)
	assert.False(t, body.read)
}
