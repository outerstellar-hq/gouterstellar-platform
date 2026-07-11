package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"template/base.html": &fstest.MapFile{
			Data: []byte(`{{ define "base" }}<html><head><title>{{ .Title }}</title></head><body><nav>NAV</nav><main>{{ template "content" .BodyData }}</main></body></html>{{ end }}`),
		},
		"template/partials/pagination.html": &fstest.MapFile{
			Data: []byte(`{{ define "pagination" }}<div class="pagination">page {{ .CurrentPage }}</div>{{ end }}`),
		},
		"template/pages/home.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>{{ .Title }}</h1>{{ end }}`),
		},
	}
}

func TestNewRendererParsesAllPages(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	assert.Contains(t, r.pages, "home", "home page should be parsed")
}

func TestRenderPageProducesShellAndContent(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err = r.RenderPage(rec, req, "home", map[string]string{"Title": "Welcome"})
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "<nav>NAV</nav>", "shell nav should be present")
	assert.Contains(t, body, "<h1>Welcome</h1>", "page content should be present")
}

func TestRenderPartialProducesFragment(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	err = r.RenderPartial(rec, "pagination", map[string]int{"CurrentPage": 3})
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "page 3")
	assert.NotContains(t, body, "<nav>", "partial should not contain shell chrome")
}

func TestRenderPageSetsContentType(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	_ = r.RenderPage(rec, req, "home", map[string]string{"Title": "X"})

	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
}
