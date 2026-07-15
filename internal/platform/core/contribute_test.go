package core

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

var stub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

// stubContributor registers a single route in a chosen group so tests can
// cover each group without depending on real handlers.
type stubContributor struct {
	group    extplatform.RouteGroup
	method   string
	pattern  string
	desc     string
	handler  http.Handler
	skipBody bool
}

func (s stubContributor) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	if s.skipBody {
		return nil
	}
	switch s.group {
	case extplatform.GroupPublicUI:
		ctx.Routes.Public(s.method, s.pattern, s.desc, s.handler)
	case extplatform.GroupProtectedUI:
		ctx.Routes.Protected(s.method, s.pattern, s.desc, s.handler)
	case extplatform.GroupAPI:
		ctx.Routes.API(s.method, s.pattern, s.desc, s.handler)
	case extplatform.GroupAdmin:
		ctx.Routes.Admin(s.method, s.pattern, s.desc, s.handler)
	}
	return nil
}

func TestCoreContributesAllRouteGroups(t *testing.T) {
	ext := NewExtension()
	ext.SetHealth(stub)
	ext.SetMetrics(stub)
	ext.SetStatic(stub)
	ext.SetOpenAPI(stub)
	ext.AddContributors(
		stubContributor{group: extplatform.GroupPublicUI, method: http.MethodGet, pattern: "/auth", desc: "login", handler: stub},
		stubContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/", desc: "home", handler: stub},
		stubContributor{group: extplatform.GroupAPI, method: http.MethodGet, pattern: "/api/v1/sync", desc: "sync", handler: stub},
		stubContributor{group: extplatform.GroupAdmin, method: http.MethodGet, pattern: "/admin/users", desc: "users", handler: stub},
	)

	ctx := extplatform.NewContributionContext(ext.Manifest().ID)
	err := ext.Contribute(ctx)
	require.NoError(t, err)

	routes := ctx.Routes.All()
	groups := map[extplatform.RouteGroup]bool{}
	for _, r := range routes {
		groups[r.Group] = true
	}

	assert.True(t, groups[extplatform.GroupPublicUI], "should have public UI routes")
	assert.True(t, groups[extplatform.GroupProtectedUI], "should have protected UI routes")
	assert.True(t, groups[extplatform.GroupAPI], "should have API routes")
	assert.True(t, groups[extplatform.GroupAdmin], "should have admin routes")
}

func TestCoreManifest(t *testing.T) {
	ext := NewExtension()
	m := ext.Manifest()

	assert.Equal(t, "platform-core", m.ID)
	assert.Equal(t, extplatform.FullPlatform, m.Mode)
	assert.NotEmpty(t, m.Ownership.UI)
	assert.NotEmpty(t, m.Ownership.API)
	assert.NotEmpty(t, m.Ownership.Admin)
}

func TestCoreNavigationItems(t *testing.T) {
	ext := NewExtension()
	ext.SetHealth(stub)
	ext.SetMetrics(stub)
	ext.SetStatic(stub)
	ext.SetOpenAPI(stub)
	ext.AddContributors(stubContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/", desc: "home", handler: stub})
	ctx := extplatform.NewContributionContext(ext.Manifest().ID)
	err := ext.Contribute(ctx)
	require.NoError(t, err)

	nav := ctx.Navigation.Items()
	labels := make([]string, len(nav))
	for i, item := range nav {
		labels[i] = item.Label
	}

	assert.Contains(t, labels, "Home")
	assert.Contains(t, labels, "Contacts")
}
