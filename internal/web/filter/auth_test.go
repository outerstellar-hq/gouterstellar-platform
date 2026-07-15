package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/security"
)

func TestBearerAuthRejectsInvalidCredentials(t *testing.T) {
	realm := security.NewApiKeyRealm(func(string) *model.User { return nil })
	handler := BearerAuth(realm)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, authorization := range []string{"Basic abc", "Bearer invalid"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
	}
}
