package platform_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/platform/core"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

type routeContributor struct {
	group   extplatform.RouteGroup
	method  string
	pattern string
	label   string
}

func (c routeContributor) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(c.label))
	})
	switch c.group {
	case extplatform.GroupPublicUI:
		ctx.Routes.Public(c.method, c.pattern, c.label, handler)
	case extplatform.GroupProtectedUI:
		ctx.Routes.Protected(c.method, c.pattern, c.label, handler)
	case extplatform.GroupAdmin:
		ctx.Routes.Admin(c.method, c.pattern, c.label, handler)
	}
	return nil
}

type repoQualityExtension struct {
	homePath string
	pages    []extplatform.PlatformPageSet
	renderer *extplatform.PageRegistry
}

func (e *repoQualityExtension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "repoquality",
		Label: "RepoQuality",
		Mode:  extplatform.ExtensionHost,
		Ownership: extplatform.RouteOwnership{
			UI: []string{e.homePath},
		},
	}
}

func (e *repoQualityExtension) Contribute(ctx *extplatform.ContributionContext) error {
	ctx.PlatformPages.Include(e.pages...)
	if err := ctx.Pages.Register(extplatform.TemplateSource{
		FS: fstest.MapFS{
			"pages/repoquality.html": &fstest.MapFile{Data: []byte(`{{ define "content" }}<main id="extension-home">RepoQuality</main>{{ end }}`)},
		},
		PagesDir: "pages",
	}); err != nil {
		return err
	}
	e.renderer = ctx.Pages
	ctx.Routes.Protected(http.MethodGet, e.homePath, "RepoQuality home", http.HandlerFunc(e.home))
	ctx.Navigation.Add("RepoQuality", e.homePath, "quality")
	return nil
}

func (e *repoQualityExtension) home(w http.ResponseWriter, r *http.Request) {
	if err := e.renderer.Render(w, r, "repoquality", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TestExtensionHostHidesCorePagesAndChromeByDefault(t *testing.T) {
	handler := newExtensionHostTestHandler(t, &repoQualityExtension{homePath: "/repoquality"})

	response := serveWithUser(handler, http.MethodGet, "/repoquality")
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `id="extension-home"`)
	assert.Contains(t, body, `href="/repoquality"`)
	assert.NotContains(t, body, `href="/settings"`)
	assert.NotContains(t, body, `href="/contacts"`)
	assert.NotContains(t, body, `href="/search"`)
	assert.NotContains(t, body, `action="/search"`)
	assert.NotContains(t, body, `href="/notifications"`)
	assert.NotContains(t, body, `href="/auth/profile"`)
	assert.NotContains(t, body, `href="/admin/users"`)

	for _, path := range []string{"/settings", "/contacts", "/search", "/notifications", "/messages/trash", "/auth/profile", "/admin/users", "/admin/dev"} {
		assert.Equal(t, http.StatusNotFound, serveWithUser(handler, http.MethodGet, path).Code, path)
	}
}

func TestExtensionHostCanOptIntoSelectedCorePagesAndChrome(t *testing.T) {
	handler := newExtensionHostTestHandler(t, &repoQualityExtension{
		homePath: "/repoquality",
		pages: []extplatform.PlatformPageSet{
			extplatform.PlatformPageHome,
			extplatform.PlatformPageSearch,
			extplatform.PlatformPageSettings,
			extplatform.PlatformPageNotifications,
			extplatform.PlatformPageProfile,
			extplatform.PlatformPageAdmin,
		},
	})

	response := serveWithUser(handler, http.MethodGet, "/repoquality")
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `href="/"`)
	assert.Contains(t, body, `href="/messages/trash"`)
	assert.Contains(t, body, `href="/search"`)
	assert.Contains(t, body, `action="/search"`)
	assert.Contains(t, body, `href="/settings"`)
	assert.Contains(t, body, `id="notification-bell"`)
	assert.Contains(t, body, `hx-get="/components/notification-bell`)
	assert.Contains(t, body, `href="/auth/profile"`)
	assert.Contains(t, body, `href="/admin/users"`)
	assert.NotContains(t, body, `href="/contacts"`)

	for _, path := range []string{"/settings", "/search", "/notifications", "/auth/profile", "/admin/users"} {
		assert.NotEqual(t, http.StatusNotFound, serveWithUser(handler, http.MethodGet, path).Code, path)
	}
	assert.Equal(t, http.StatusNotFound, serveWithUser(handler, http.MethodGet, "/contacts").Code)
	assert.Equal(t, http.StatusNotFound, serveWithUser(handler, http.MethodGet, "/admin/dev").Code)
}

func TestExtensionHostCanLetExtensionOwnRoot(t *testing.T) {
	handler := newExtensionHostTestHandler(t, &repoQualityExtension{homePath: "/"})

	response := serveWithUser(handler, http.MethodGet, "/")
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `id="extension-home"`)
	assert.Contains(t, body, `href="/"`)
	assert.NotContains(t, body, `href="/repoquality"`)
	assert.Equal(t, http.StatusNotFound, serveWithUser(handler, http.MethodGet, "/repoquality").Code)
}

func newExtensionHostTestHandler(t *testing.T, extension extplatform.Extension) http.Handler {
	t.Helper()
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	require.NoError(t, err)

	coreExtension := core.NewExtension()
	stub := http.NotFoundHandler()
	coreExtension.SetOperations(stub, stub, stub, stub)
	coreExtension.SetDiagnostics(stub)
	coreExtension.SetMetrics(stub)
	coreExtension.SetStatic(fstest.MapFS{
		"css/main.css": &fstest.MapFile{Data: []byte("body{}")},
		"swagger.html": &fstest.MapFile{Data: []byte("<html></html>")},
	})
	coreExtension.SetOpenAPI(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	})
	coreExtension.AddContributors(
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/", label: "home"},
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/messages/trash", label: "trash"},
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/contacts", label: "contacts"},
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/search", label: "search"},
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/settings", label: "settings"},
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/notifications", label: "notifications"},
		routeContributor{group: extplatform.GroupProtectedUI, method: http.MethodGet, pattern: "/auth/profile", label: "profile"},
		routeContributor{group: extplatform.GroupAdmin, method: http.MethodGet, pattern: "/admin/users", label: "users"},
		routeContributor{group: extplatform.GroupAdmin, method: http.MethodGet, pattern: "/admin/dev", label: "dev"},
	)

	handler, err := extplatform.NewHandler(extplatform.Options{
		Mode:       extplatform.ExtensionHost,
		Extensions: []extplatform.Extension{coreExtension, extension},
		Services:   extplatform.ServiceBag{Pages: renderer},
		MiddlewareChain: []func(http.Handler) http.Handler{
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, web.WithUser(r, &model.User{ID: uuid.New(), Username: "admin", Role: model.RoleAdmin}))
				})
			},
		},
	})
	require.NoError(t, err)
	return handler
}

func serveWithUser(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
	return response
}
