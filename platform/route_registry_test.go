package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteRegistryRegistration(t *testing.T) {
	reg := newRouteRegistry("reports")

	reg.Protected(http.MethodGet, "/reports", "Reports home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	reg.API(http.MethodGet, "/api/v1/reports/summary", "Summary", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	routes := reg.All()
	require.Len(t, routes, 2)
	assert.Equal(t, "reports", routes[0].Owner)
	assert.Equal(t, GroupProtectedUI, routes[0].Group)
	assert.Equal(t, GroupAPI, routes[1].Group)
}

func TestRouteRegistryOwnerStamping(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/reports", "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, r := range reg.All() {
		assert.Equal(t, "reports", r.Owner, "owner should be stamped from registry construction")
	}
}

func TestValidateAbsolutePaths(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "relative/path", "", stubHandler())

	errs := validateRoutes(reg.All(), FullPlatform, nil)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "must be absolute")
}

func TestValidateOwnership(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/settings", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}},
	}
	errs := validateRoutes(reg.All(), FullPlatform, ownership)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "outside ownership")
}

func TestValidateConflictDetection(t *testing.T) {
	reg1 := newRouteRegistry("platform-core")
	reg1.Protected(http.MethodGet, "/reports", "", stubHandler())

	reg2 := newRouteRegistry("reports")
	reg2.Protected(http.MethodGet, "/reports", "", stubHandler())

	all := append(reg1.All(), reg2.All()...)
	ownership := map[string]RouteOwnership{
		"platform-core": {UI: []string{"/"}},
		"reports":       {UI: []string{"/reports"}},
	}
	errs := validateRoutes(all, FullPlatform, ownership)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "route conflict: GET /reports")
	assert.Contains(t, errs[0].Error(), "platform-core")
	assert.Contains(t, errs[0].Error(), "reports")
}

func TestValidateHeadlessRejectsHTML(t *testing.T) {
	reg := newRouteRegistry("platform-core")
	reg.Protected(http.MethodGet, "/", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"platform-core": {UI: []string{"/"}},
	}
	errs := validateRoutes(reg.All(), Headless, ownership)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "headless mode rejects HTML route")
}

func TestValidateCollectsAllErrors(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "relative", "", stubHandler())
	reg.Protected(http.MethodGet, "/outside", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}},
	}
	errs := validateRoutes(reg.All(), FullPlatform, ownership)
	assert.Len(t, errs, 2, "should collect all errors, not abort on first")
}

func TestValidateAcceptsOwnedRoute(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/reports/home", "", stubHandler())
	reg.API(http.MethodGet, "/api/v1/reports/summary", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}, API: []string{"/api/v1", "/api/v1/reports"}},
	}
	errs := validateRoutes(reg.All(), FullPlatform, ownership)
	assert.Empty(t, errs)
}

func stubHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func TestBuildMountsRoutes(t *testing.T) {
	reg := newRouteRegistry("reports")
	called := false
	reg.Protected(http.MethodGet, "/reports", "home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}},
	}

	r := chi.NewRouter()
	mounted := buildRoutes(r, reg.All(), FullPlatform, ownership)
	require.Len(t, mounted, 1)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, called, "handler should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBuildHeadlessDropsHTMLRoutes(t *testing.T) {
	reg := newRouteRegistry("reports")
	reg.Protected(http.MethodGet, "/reports", "", stubHandler())
	reg.API(http.MethodGet, "/api/v1/reports/summary", "", stubHandler())

	ownership := map[string]RouteOwnership{
		"reports": {UI: []string{"/reports"}, API: []string{"/api/v1"}},
	}

	r := chi.NewRouter()
	mounted := buildRoutes(r, reg.All(), Headless, ownership)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "HTML route should not be mounted in headless mode")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code, "API route should be mounted in headless mode")

	_ = mounted
}
