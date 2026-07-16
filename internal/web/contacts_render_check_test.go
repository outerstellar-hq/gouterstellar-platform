package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

// TestContactsPageRendersCRUDWithCSRF verifies the contacts page renders the
// create form and a per-contact delete button carrying the CSRF token at render
// time (parse-time success does not guarantee runtime field resolution, and the
// delete form relies on $.CSRFToken resolving inside a range over the page data).
func TestContactsPageRendersCRUDWithCSRF(t *testing.T) {
	r, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/contacts", nil)
	req = web.WithCSRFToken(req, "TOKEN-123")
	rec := httptest.NewRecorder()

	err = r.RenderPage(rec, req, "contacts", viewmodel.ContactsPage{
		Contacts: []viewmodel.ContactItem{
			{SyncID: "srv_abc", Name: "Alice", Company: "Acme"},
		},
	})
	if err != nil {
		t.Fatalf("render contacts: %v", err)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`name="csrf_token" value="TOKEN-123"`,
		`action="/contacts"`,
		`action="/contacts/srv_abc/delete"`,
		`href="/contacts/srv_abc/edit"`,
		`Delete`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
}
