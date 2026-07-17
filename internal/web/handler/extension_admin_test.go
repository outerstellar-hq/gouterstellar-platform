package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

type dashboardExtension struct{}

func (dashboardExtension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "reports",
		Label: "Reports App",
		Mode:  extplatform.ExtensionHost,
		Ownership: extplatform.RouteOwnership{
			UI:     []string{"/extension/reports"},
			Admin:  []string{"/admin/reports"},
			Assets: []string{"/extensions/reports/assets"},
		},
	}
}

func (dashboardExtension) Contribute(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/extension/reports", "Reports UI", textHandler("reports"))
	ctx.Routes.Admin(http.MethodGet, "/admin/reports", "Reports admin", textHandler("admin reports"))
	ctx.Routes.Assets("/extensions/reports/assets/*", http.StripPrefix("/extensions/reports/assets/", textHandler("asset")))
	ctx.Readiness.Up("reports-cache", "Reports cache is ready")
	return nil
}

func TestExtensionDashboardShowsDiagnostics(t *testing.T) {
	catalog := extplatform.NewCatalog()
	_, err := extplatform.NewHandler(extplatform.Options{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{dashboardExtension{}},
		Catalog:    catalog,
	})
	require.NoError(t, err)
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "dev")
	require.NoError(t, err)

	handler := NewExtensionAdminHandler(catalog, renderer)
	response := httptest.NewRecorder()
	handler.Extensions(response, httptest.NewRequest(http.MethodGet, "/admin/extensions", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "Extension diagnostics")
	assert.Contains(t, body, "Reports App")
	assert.Contains(t, body, "reports")
	assert.Contains(t, body, "/extension/reports")
	assert.Contains(t, body, "/admin/reports")
	assert.Contains(t, body, "/extensions/reports/assets")
	assert.Contains(t, body, "/extensions/reports/assets/*")
	assert.Contains(t, body, "Admin sections")
	assert.Contains(t, body, "Routes and assets")
	assert.Contains(t, body, "reports-cache")
	assert.Contains(t, body, "Reports cache is ready")
}

func TestExtensionAdminIndexRedirectsToDashboard(t *testing.T) {
	handler := NewExtensionAdminHandler(extplatform.NewCatalog(), nil)
	response := httptest.NewRecorder()

	handler.Index(response, httptest.NewRequest(http.MethodGet, "/admin", nil))

	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "/admin/extensions", response.Header().Get("Location"))
}

func textHandler(text string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, text)
	})
}
