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

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name string
		user *model.User
		want int
	}{
		{name: "anonymous", want: http.StatusUnauthorized},
		{name: "regular user", user: &model.User{ID: uuid.New(), Role: model.RoleUser}, want: http.StatusForbidden},
		{name: "administrator", user: &model.User{ID: uuid.New(), Role: model.RoleAdmin}, want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
			if tt.user != nil {
				req = req.WithContext(web.WithUser(req, tt.user).Context())
			}
			res := httptest.NewRecorder()
			RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(res, req)
			assert.Equal(t, tt.want, res.Code)
		})
	}
}
