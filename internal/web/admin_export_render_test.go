package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

func TestAdminPagesExposeCSVAndJSONExports(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	tests := []struct {
		page string
		data any
		csv  string
		json string
	}{
		{"admin_users", viewmodel.AdminUsersPage{}, "/admin/users/export", "/admin/users/export/json"},
		{"admin_audit", viewmodel.AdminAuditPage{}, "/admin/audit/export", "/admin/audit/export/json"},
	}
	for _, test := range tests {
		t.Run(test.page, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			err := renderer.RenderPage(recorder, httptest.NewRequest(http.MethodGet, "/admin", nil), test.page, test.data)
			if err != nil {
				t.Fatalf("render page: %v", err)
			}
			body := recorder.Body.String()
			if !strings.Contains(body, `href="`+test.csv+`"`) || !strings.Contains(body, `href="`+test.json+`"`) {
				t.Errorf("page does not expose both export formats: %s", body)
			}
		})
	}
}
