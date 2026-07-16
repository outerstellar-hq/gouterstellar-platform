package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

func TestAuthenticatedShellUsesSavedLayoutAndSemanticLandmarks(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	for _, layout := range []string{"sidebar", "topbar"} {
		t.Run(layout, func(t *testing.T) {
			req := web.WithUser(httptest.NewRequest(http.MethodGet, "/", nil), &model.User{
				ID: uuid.New(), Username: "alex", Role: model.RoleUser, Layout: &layout,
			})
			req = web.WithNavItems(req, []viewmodel.NavItem{{Label: "Home", URL: "/"}})
			recorder := httptest.NewRecorder()
			if err := renderer.RenderPage(recorder, req, "home", map[string]any{}); err != nil {
				t.Fatalf("render shell: %v", err)
			}

			body := recorder.Body.String()
			for _, want := range []string{
				`class="layout-` + layout + `"`,
				`class="app-shell"`,
				`class="app-nav" aria-label="Primary navigation"`,
				`class="nav-link active" aria-current="page"`,
				`class="skip-link" href="#main-content"`,
				`id="main-content" class="app-main" tabindex="-1"`,
				`class="app-footer"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("rendered %s shell missing %q", layout, want)
				}
			}
		})
	}
}

func TestPublicShellStaysInSingleColumnWithoutNavigation(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := renderer.RenderPage(recorder, httptest.NewRequest(http.MethodGet, "/auth", nil), "auth_login", viewmodel.AuthPage{}); err != nil {
		t.Fatalf("render public shell: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `class="app-shell app-shell-public"`) {
		t.Error("public shell is missing its single-column layout override")
	}
	if strings.Contains(body, `class="app-nav"`) {
		t.Error("public shell unexpectedly renders authenticated navigation")
	}
}
