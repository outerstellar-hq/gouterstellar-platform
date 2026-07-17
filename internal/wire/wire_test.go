package wire

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/extensions/reports"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/config"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/filter"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
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
	coreExtension.SetStatic(fstest.MapFS{
		"css/main.css": &fstest.MapFile{Data: []byte("body {}")},
		"swagger.html": &fstest.MapFile{Data: []byte("<html></html>")},
	})

	handler, err := extplatform.NewHandler(extplatform.Options{
		Mode:     extplatform.FullPlatform,
		Services: app.ServiceBag,
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
	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "/auth?returnTo=%2F", response.Header().Get("Location"))
}
