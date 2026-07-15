package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

func TestRendererProducesCompleteNonEmptyPage(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap())
	require.NoError(t, err)
	response := httptest.NewRecorder()
	require.NoError(t, renderer.Render(response, "home.html", viewmodel.HomePage{}))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Welcome to Outerstellar Platform")
	assert.Contains(t, response.Body.String(), "/static/js/platform.js")
}

func TestRendererRejectsMissingTemplate(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap())
	require.NoError(t, err)
	err = renderer.Render(httptest.NewRecorder(), "missing.html", nil)
	assert.ErrorContains(t, err, "does not exist")
}

func TestRendererPreservesErrorStatus(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap())
	require.NoError(t, err)
	response := httptest.NewRecorder()
	require.NoError(t, renderer.RenderWithStatus(response, "error.html", viewmodel.ErrorPage{StatusCode: 404, Title: "Missing"}, http.StatusNotFound))
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Body.String(), "404 - Missing")
}
