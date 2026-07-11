package core

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

var stub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

func TestCoreContributesAllRouteGroups(t *testing.T) {
	// Populate enough fields to cover every route group
	b := Bundle{
		// PublicUI
		AuthShowLogin: stub,
		// ProtectedUI
		HomeShow: stub,
		// API
		SyncPullMessages: stub,
		// Admin
		AdminListUsers:     stub,
		DevDashboardEnabled: true,
		DevDashboard:       stub,
	}
	ext := NewExtension(b)

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
	ext := NewExtension(Bundle{})
	m := ext.Manifest()

	assert.Equal(t, "platform-core", m.ID)
	assert.Equal(t, extplatform.FullPlatform, m.Mode)
	assert.NotEmpty(t, m.Ownership.UI)
	assert.NotEmpty(t, m.Ownership.API)
	assert.NotEmpty(t, m.Ownership.Admin)
}

func TestCoreNavigationItems(t *testing.T) {
	ext := NewExtension(Bundle{HomeShow: stub, ContactsList: stub, SearchSearch: stub, SettingsShow: stub, NotifsList: stub})
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
