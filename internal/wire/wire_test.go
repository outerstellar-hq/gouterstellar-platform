package wire

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/extensions/reports"
	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

func TestProductionExtensionsAssemble(t *testing.T) {
	cfg := config.Load()
	app := Wire(cfg, nil, web.TemplateFS())

	catalog := extplatform.NewCatalog()
	coreExtension := BuildCoreExtension(app, catalog)
	stub := http.NotFoundHandler()
	coreExtension.SetOperations(stub, stub, stub, stub)
	coreExtension.SetDiagnostics(stub)
	coreExtension.SetMetrics(http.NotFoundHandler())
	coreExtension.SetStatic(http.NotFoundHandler())

	handler, err := extplatform.NewHandler(extplatform.Options{
		Mode: extplatform.FullPlatform,
		Extensions: []extplatform.Extension{
			coreExtension,
			reports.New(app.ServiceBag.MessageCounter),
		},
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			extplatform.GroupProtectedUI: {filter.RequireAuthenticated},
		},
		Catalog: catalog,
	})
	require.NoError(t, err)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/auth", response.Header().Get("Location"))
}
