package reports

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

func TestReportsHomeHTTP(t *testing.T) {
	ext := New(stubMessageCounter{count: 99})
	app, err := newReportsTestApp(ext, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Reports")
	assert.Contains(t, rec.Body.String(), "99")
	assert.Contains(t, rec.Body.String(), `class="app-shell app-shell-public"`)
	assert.Contains(t, rec.Body.String(), "vtest-build")
}

func TestReportsHomeUsesAuthenticatedPlatformShell(t *testing.T) {
	ext := New(stubMessageCounter{count: 12})
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = web.WithUser(r, &model.User{ID: uuid.New(), Username: "operator", Role: model.RoleUser})
			next.ServeHTTP(w, web.WithCSRFToken(r, "extension-csrf"))
		})
	}
	app, err := newReportsTestApp(ext, middleware)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	app.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `href="/reports"`)
	assert.Contains(t, body, `operator`)
	assert.Contains(t, body, `Viewing as operator.`)
	assert.Contains(t, body, `content="extension-csrf"`)
	assert.Contains(t, body, `hx-ext="ws" ws-connect="/ws/sync"`)
}

func TestReportsSummaryAPI(t *testing.T) {
	ext := New(stubMessageCounter{count: 7})
	app, err := newReportsTestApp(ext, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "message_count")
	assert.Contains(t, rec.Body.String(), "7")
}

func newReportsTestApp(ext *Extension, middleware func(http.Handler) http.Handler) (*extplatform.TestApp, error) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test-build")
	if err != nil {
		return nil, err
	}
	var middlewareChain []func(http.Handler) http.Handler
	if middleware != nil {
		middlewareChain = append(middlewareChain, middleware)
	}
	return extplatform.NewTestApp(extplatform.TestOptions{
		Mode:            extplatform.FullPlatform,
		Extensions:      []extplatform.Extension{ext},
		Services:        extplatform.ServiceBag{Pages: renderer},
		MiddlewareChain: middlewareChain,
	})
}
