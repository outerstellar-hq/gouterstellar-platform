package handler

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestDesktopClientContractRoutes(t *testing.T) {
	router := chi.NewRouter()
	(&AuthAPI{}).RegisterRoutes(router)
	(&UserAdminAPI{}).RegisterRoutes(router)
	(&DeviceRegistrationAPI{}).RegisterRoutes(router)

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/register"},
		{http.MethodPut, "/api/v1/auth/password"},
		{http.MethodPost, "/api/v1/auth/reset-request"},
		{http.MethodPost, "/api/v1/auth/reset-confirm"},
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPut, "/api/v1/admin/users/8c607f06-69cf-4170-af4e-6a2bfcf43eae/enabled"},
		{http.MethodPut, "/api/v1/admin/users/8c607f06-69cf-4170-af4e-6a2bfcf43eae/role"},
		{http.MethodPost, "/api/v1/devices/register"},
		{http.MethodDelete, "/api/v1/devices/register"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			context := chi.NewRouteContext()
			assert.True(t, router.Match(context, route.method, route.path))
		})
	}
}

func TestWebNavigationRoutes(t *testing.T) {
	router := chi.NewRouter()
	(&HomeHandler{}).RegisterRoutes(router)
	(&UserAdminHandler{}).RegisterRoutes(router)

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/messages"},
		{http.MethodGet, "/messages/trash"},
		{http.MethodPost, "/admin/users/8c607f06-69cf-4170-af4e-6a2bfcf43eae/enabled"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			context := chi.NewRouteContext()
			assert.True(t, router.Match(context, route.method, route.path))
		})
	}
}
