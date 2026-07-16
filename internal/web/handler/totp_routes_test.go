package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

func TestTOTPRouteContracts(t *testing.T) {
	ctx := extplatform.NewContributionContext("platform-core")
	require.NoError(t, (&AuthHandler{}).ContributeRoutes(ctx))
	require.NoError(t, (&AuthAPI{}).ContributeRoutes(ctx))
	require.NoError(t, (&SettingsHandler{}).ContributeRoutes(ctx))

	routes := make(map[string]extplatform.RouteGroup)
	for _, route := range ctx.Routes.All() {
		routes[route.Method+" "+route.Pattern] = route.Group
	}

	expected := map[string]extplatform.RouteGroup{
		http.MethodPost + " /auth/totp/verify":                  extplatform.GroupPublicUI,
		http.MethodPost + " /auth/components/totp-verify":       extplatform.GroupPublicUI,
		http.MethodPost + " /api/v1/auth/totp/verify":           extplatform.GroupAPI,
		http.MethodPost + " /api/v1/auth/totp/setup":            extplatform.GroupAPI,
		http.MethodPost + " /api/v1/auth/totp/confirm":          extplatform.GroupAPI,
		http.MethodPost + " /api/v1/auth/totp/disable":          extplatform.GroupAPI,
		http.MethodPost + " /settings/totp/setup":               extplatform.GroupProtectedUI,
		http.MethodPost + " /settings/totp/confirm":             extplatform.GroupProtectedUI,
		http.MethodPost + " /settings/totp/disable":             extplatform.GroupProtectedUI,
		http.MethodGet + " /auth/components/totp-setup-status":  extplatform.GroupProtectedUI,
		http.MethodPost + " /auth/components/totp-setup":        extplatform.GroupProtectedUI,
		http.MethodPost + " /auth/components/totp-verify-setup": extplatform.GroupProtectedUI,
		http.MethodPost + " /auth/components/totp-disable":      extplatform.GroupProtectedUI,
		http.MethodPut + " /api/v1/auth/password":               extplatform.GroupAPI,
		http.MethodPost + " /api/v1/auth/reset-request":         extplatform.GroupAPI,
		http.MethodPost + " /api/v1/auth/reset-confirm":         extplatform.GroupAPI,
	}
	for route, group := range expected {
		assert.Equal(t, group, routes[route], route)
	}
}
