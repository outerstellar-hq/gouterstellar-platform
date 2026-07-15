package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

func TestRequireAuthenticated(t *testing.T) {
	handler := RequireAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil))
	assert.Equal(t, http.StatusUnauthorized, anonymous.Code)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	request = web.WithUser(request, &model.User{ID: uuid.New(), Role: model.RoleUser})
	authenticated := httptest.NewRecorder()
	handler.ServeHTTP(authenticated, request)
	assert.Equal(t, http.StatusNoContent, authenticated.Code)
}
