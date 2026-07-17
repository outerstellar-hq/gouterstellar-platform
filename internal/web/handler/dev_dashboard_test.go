package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

func TestDevDashboardContributesRoutesOnlyWhenEnabled(t *testing.T) {
	disabled := NewDevDashboardHandler(nil, nil, nil, nil, false)
	disabledContext := extplatform.NewContributionContext("platform-core")
	require.NoError(t, disabled.ContributeRoutes(disabledContext))
	assert.Empty(t, disabledContext.Routes.All())

	enabled := NewDevDashboardHandler(nil, nil, nil, nil, true)
	enabledContext := extplatform.NewContributionContext("platform-core")
	require.NoError(t, enabled.ContributeRoutes(enabledContext))

	routes := make(map[string]extplatform.RouteGroup)
	for _, route := range enabledContext.Routes.All() {
		routes[route.Method+" "+route.Pattern] = route.Group
	}
	assert.Equal(t, extplatform.GroupAdmin, routes[http.MethodGet+" /admin/dev"])
	assert.Equal(t, extplatform.GroupAdmin, routes[http.MethodPost+" /admin/dev/outbox/process"])
	assert.Equal(t, extplatform.GroupAdmin, routes[http.MethodPost+" /admin/dev/sessions/cleanup"])
	assert.Equal(t, extplatform.GroupAdmin, routes[http.MethodPost+" /admin/dev/cache/invalidate"])
}
