package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

// TestRendererParsesAllRealTemplates builds a renderer from the real embedded
// template FS and verifies every template parses without error. This catches
// syntax errors, missing partials, and broken function references across the
// whole template set.
func TestRendererParsesAllRealTemplates(t *testing.T) {
	templateFS := web.TemplateFS()
	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), "test")
	require.NoError(t, err, "all embedded templates must parse without error")
	assert.NotNil(t, renderer)
}

// TestRendererHasAllExpectedPages verifies the renderer has a page entry for
// every page name that handlers render. If a handler references a page that
// has no template, RenderPage would return an error at request time.
func TestRendererHasAllExpectedPages(t *testing.T) {
	templateFS := web.TemplateFS()
	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), "test")
	require.NoError(t, err)

	// Every page that handlers render must exist in the renderer.
	expectedPages := []string{
		"contacts", "notifications", "settings",
		"admin_users", "admin_audit", "admin_extensions", "error",
		"auth_login", "auth_change_password",
		"auth_reset_password", "auth_reset_sent",
		"search", "dev_dashboard", "messages", "message_conflict",
	}

	for _, page := range expectedPages {
		assert.True(t, renderer.HasPage(page), "renderer should have page %q", page)
	}
}
