package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type requestContextExtension struct{}

func (requestContextExtension) Manifest() Manifest {
	return Manifest{
		ID:   "request-context-test",
		Mode: FullPlatform,
		Ownership: RouteOwnership{
			UI:    []string{"/context"},
			Admin: []string{"/admin/context"},
		},
	}
}

func (requestContextExtension) Contribute(ctx *ContributionContext) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RequestContextFrom(r))
	})
	ctx.Routes.Public(http.MethodGet, "/context", "Public context", handler)
	ctx.Routes.Protected(http.MethodGet, "/context/protected", "Protected context", handler)
	ctx.Routes.Admin(http.MethodGet, "/admin/context", "Admin context", handler)
	return nil
}

func TestAssembledRequestContextProjectsOnlyPublicIdentity(t *testing.T) {
	userID := uuid.New()
	injectIdentity := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = web.WithCSRFToken(r, "extension-csrf")
			if username := r.Header.Get("X-Test-User"); username != "" {
				role := model.RoleUser
				if r.Header.Get("X-Test-Admin") == "true" {
					role = model.RoleAdmin
				}
				r = web.WithUser(r, &model.User{ID: userID, Username: username, Role: role})
			}
			next.ServeHTTP(w, r)
		})
	}
	requireAdmin := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := web.UserFromRequest(r)
			if user == nil || user.Role != model.RoleAdmin {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	handler, err := NewHandler(Options{
		Mode:            FullPlatform,
		Extensions:      []Extension{requestContextExtension{}},
		MiddlewareChain: []func(http.Handler) http.Handler{injectIdentity},
		GroupMiddleware: map[RouteGroup][]func(http.Handler) http.Handler{
			GroupAdmin: {requireAdmin},
		},
	})
	require.NoError(t, err)

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/context", nil))
	require.Equal(t, http.StatusOK, anonymous.Code)
	assert.Contains(t, anonymous.Body.String(), `"User":null`)
	assert.Contains(t, anonymous.Body.String(), `"CSRFToken":"extension-csrf"`)

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "/context/protected", nil)
	authenticatedRequest.Header.Set("X-Test-User", "operator")
	authenticated := httptest.NewRecorder()
	handler.ServeHTTP(authenticated, authenticatedRequest)
	require.Equal(t, http.StatusOK, authenticated.Code)
	body := authenticated.Body.String()
	assert.Contains(t, body, userID.String())
	assert.Contains(t, body, `"Username":"operator"`)
	assert.Contains(t, body, `"Role":"USER"`)
	assert.Contains(t, body, `"IsAdmin":false`)
	assert.NotContains(t, strings.ToLower(body), "password")
	assert.NotContains(t, strings.ToLower(body), "session")
	assert.NotContains(t, strings.ToLower(body), "cookie")

	ordinaryAdminRequest := httptest.NewRequest(http.MethodGet, "/admin/context", nil)
	ordinaryAdminRequest.Header.Set("X-Test-User", "operator")
	ordinaryAdmin := httptest.NewRecorder()
	handler.ServeHTTP(ordinaryAdmin, ordinaryAdminRequest)
	require.Equal(t, http.StatusForbidden, ordinaryAdmin.Code)

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin/context", nil)
	adminRequest.Header.Set("X-Test-User", "administrator")
	adminRequest.Header.Set("X-Test-Admin", "true")
	admin := httptest.NewRecorder()
	handler.ServeHTTP(admin, adminRequest)
	require.Equal(t, http.StatusOK, admin.Code)
	assert.Contains(t, admin.Body.String(), `"IsAdmin":true`)
}
