package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

func TestRequireAuthenticated(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireAuthenticated(next)

	t.Run("browser redirects to login", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusFound, response.Code)
		assert.Equal(t, "/auth?returnTo=%2F", response.Header().Get("Location"))
	})

	t.Run("json client receives unauthorized", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})

	t.Run("authenticated user continues", func(t *testing.T) {
		request := web.WithUser(httptest.NewRequest(http.MethodGet, "/", nil), &model.User{ID: uuid.New()})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNoContent, response.Code)
	})
}
