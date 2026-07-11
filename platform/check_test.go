package platform

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckExtensionValid(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "reports", Label: "Reports", Mode: ExtensionHost,
			Ownership: RouteOwnership{UI: []string{"/reports"}, API: []string{"/api/v1/reports"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/reports", "home", stubHandler())
			ctx.Routes.API(http.MethodGet, "/api/v1/reports/summary", "summary", stubHandler())
			ctx.Navigation.Add("Reports", "/reports", "bar-chart")
			return nil
		},
	}

	diag, err := CheckExtension(ext, TestHostContext())
	require.NoError(t, err)

	assert.Equal(t, []string{"GET /reports", "GET /api/v1/reports/summary"}, diag.RoutePatterns())
	assert.Contains(t, diag.NavigationLabels(), "Reports")
	assert.NoError(t, diag.OwnershipViolations())
}

func TestCheckExtensionOwnershipViolation(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "reports", Label: "Reports", Mode: ExtensionHost,
			Ownership: RouteOwnership{UI: []string{"/reports"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/settings", "", stubHandler()) // outside!
			return nil
		},
	}

	_, err := CheckExtension(ext, TestHostContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside ownership")
}

func TestCheckExtensionInternalDuplicate(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "reports", Label: "Reports", Mode: ExtensionHost,
			Ownership: RouteOwnership{UI: []string{"/reports"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/reports", "", stubHandler())
			ctx.Routes.Protected(http.MethodGet, "/reports", "", stubHandler()) // duplicate!
			return nil
		},
	}

	_, err := CheckExtension(ext, TestHostContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route conflict")
}
