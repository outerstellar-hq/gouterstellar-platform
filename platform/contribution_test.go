package platform

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContributionContextNavigation(t *testing.T) {
	nav := NewNavigationRegistry()
	nav.Add("Reports", "/reports", "bar-chart")
	nav.Add("Settings", "/settings", "gear")

	items := nav.Items()
	require.Len(t, items, 2)
	assert.Equal(t, "Reports", items[0].Label)
	assert.Equal(t, "/reports", items[0].URL)
	assert.Equal(t, "bar-chart", items[0].Icon)
}

func TestNewContributionForOwner(t *testing.T) {
	ctx := NewContributionContext("reports")

	ctx.Routes.Protected(http.MethodGet, "/reports", "home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ctx.Navigation.Add("Reports", "/reports", "bar-chart")

	// Verify owner stamping
	routes := ctx.Routes.All()
	require.Len(t, routes, 1)
	assert.Equal(t, "reports", routes[0].Owner)
}
