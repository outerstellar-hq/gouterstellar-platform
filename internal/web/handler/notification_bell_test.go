package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

func TestNotificationsHandlerRegistersBellRoute(t *testing.T) {
	ctx := extplatform.NewContributionContext("platform-core")
	require.NoError(t, NewNotificationsHandler(nil, nil).ContributeRoutes(ctx))

	patterns := make([]string, 0, len(ctx.Routes.All()))
	for _, route := range ctx.Routes.All() {
		patterns = append(patterns, route.Pattern)
	}
	assert.Contains(t, patterns, "/components/notification-bell")
}
