package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type bannerTestExtension struct {
	provider BannerProvider
	pages    *PageRegistry
}

func (e *bannerTestExtension) Manifest() Manifest {
	return Manifest{
		ID:        "banner-test",
		Mode:      FullPlatform,
		Ownership: RouteOwnership{UI: []string{"/banner-test"}},
	}
}

func (e *bannerTestExtension) Contribute(ctx *ContributionContext) error {
	if err := ctx.Banners.Register(e.provider); err != nil {
		return err
	}
	if err := ctx.Pages.Register(TemplateSource{
		FS: fstest.MapFS{
			"pages/banner-test.html": &fstest.MapFile{Data: []byte(`{{ define "content" }}<h1>Banner test</h1>{{ end }}`)},
		},
		PagesDir: "pages",
	}); err != nil {
		return err
	}
	e.pages = ctx.Pages
	ctx.Routes.Public(http.MethodGet, "/banner-test", "Banner test", http.HandlerFunc(e.render))
	return nil
}

func (e *bannerTestExtension) render(w http.ResponseWriter, r *http.Request) {
	if err := e.pages.Render(w, r, "banner-test", nil); err != nil {
		http.Error(w, "Failed to render banner test", http.StatusInternalServerError)
	}
}

func newBannerTestHandler(t *testing.T, provider BannerProvider) http.Handler {
	t.Helper()
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	require.NoError(t, err)

	injectIdentity := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = web.WithCSRFToken(r, "banner-csrf")
			if username := r.Header.Get("X-Test-User"); username != "" {
				role := model.RoleUser
				if r.Header.Get("X-Test-Admin") == "true" {
					role = model.RoleAdmin
				}
				r = web.WithUser(r, &model.User{ID: uuid.New(), Username: username, Role: role})
			}
			next.ServeHTTP(w, r)
		})
	}

	handler, err := NewHandler(Options{
		Mode:            FullPlatform,
		Extensions:      []Extension{&bannerTestExtension{provider: provider}},
		Services:        ServiceBag{Pages: renderer},
		MiddlewareChain: []func(http.Handler) http.Handler{injectIdentity},
	})
	require.NoError(t, err)
	return handler
}

func TestExtensionBannersRenderInPriorityOrderWithSafeDismissal(t *testing.T) {
	var receivedUser RequestUser
	provider := BannerProviderFunc(func(_ context.Context, user RequestUser) ([]Banner, error) {
		receivedUser = user
		return []Banner{
			{ID: "info", Title: "Service update", Body: "A new release is available.", Severity: BannerInfo},
			{ID: "critical", Title: "Storage unavailable", Body: "Uploads are paused.", Severity: BannerCritical, Dismissible: true, DismissURL: "/banner-test/critical/dismiss"},
		}, nil
	})
	handler := newBannerTestHandler(t, provider)

	request := httptest.NewRequest(http.MethodGet, "/banner-test", nil)
	request.Header.Set("X-Test-User", "operator")
	request.Header.Set("X-Test-Admin", "true")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Less(t, strings.Index(body, "Storage unavailable"), strings.Index(body, "Service update"))
	assert.Contains(t, body, `class="platform-banner platform-banner-critical"`)
	assert.Contains(t, body, `action="/banner-test/critical/dismiss"`)
	assert.Contains(t, body, `name="csrf_token" value="banner-csrf"`)
	assert.Contains(t, body, `aria-label="System notices"`)
	assert.Equal(t, "operator", receivedUser.Username)
	assert.True(t, receivedUser.IsAdmin)
}

func TestAnonymousShellDoesNotLoadExtensionBanners(t *testing.T) {
	calls := 0
	provider := BannerProviderFunc(func(context.Context, RequestUser) ([]Banner, error) {
		calls++
		return []Banner{{ID: "private", Title: "Private", Severity: BannerInfo}}, nil
	})
	handler := newBannerTestHandler(t, provider)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/banner-test", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "Private")
	assert.Zero(t, calls)
}

func TestBannerProviderFailureAndUnsafeDismissalFailRendering(t *testing.T) {
	tests := []struct {
		name     string
		provider BannerProvider
	}{
		{
			name: "provider failure",
			provider: BannerProviderFunc(func(context.Context, RequestUser) ([]Banner, error) {
				return nil, errors.New("control plane unavailable")
			}),
		},
		{
			name: "cross-origin dismissal",
			provider: BannerProviderFunc(func(context.Context, RequestUser) ([]Banner, error) {
				return []Banner{{ID: "unsafe", Title: "Unsafe", Severity: BannerWarning, Dismissible: true, DismissURL: "https://example.com/dismiss"}}, nil
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newBannerTestHandler(t, test.provider)
			request := httptest.NewRequest(http.MethodGet, "/banner-test", nil)
			request.Header.Set("X-Test-User", "operator")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusInternalServerError, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), "control plane unavailable")
			assert.NotContains(t, recorder.Body.String(), "https://example.com")
		})
	}
}
