package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

func TestTrashPageRendersRecoverableItemsSafely(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/messages/trash", nil)
	req = web.WithUser(req, &model.User{ID: uuid.New(), Username: "alex", Role: model.RoleUser})
	req = web.WithCSRFToken(req, "TOKEN-123")
	req = web.WithNavItems(req, []viewmodel.NavItem{
		{Label: "Messages", URL: "/messages"},
		{Label: "Trash", URL: "/messages/trash"},
	})
	recorder := httptest.NewRecorder()

	err = renderer.RenderPage(recorder, req, "trash", viewmodel.TrashPage{
		MessageList: viewmodel.MessagesPage{
			Messages: []viewmodel.MessageItem{{
				SyncID: "srv_message", Author: "<script>alert(1)</script>", Content: "Recover me", Deleted: true,
				CSRFToken: "TOKEN-123", Language: "en",
			}},
			Pagination: viewmodel.PaginationInfo{Language: "en"},
			RefreshURL: "/components/message-list?trash=true",
			Trash:      true,
		},
		ContactList: viewmodel.ContactTrashList{
			Contacts: []viewmodel.ContactItem{{
				SyncID: "srv_contact", Name: "Alice", Emails: []string{"alice@example.com"},
				CSRFToken: "TOKEN-123", Language: "en",
			}},
			Language: "en", RefreshURL: "/contacts/trash/list?lang=en",
		},
		MessageTotal: 1,
		ContactTotal: 1,
		DeletedTotal: 2,
	})
	if err != nil {
		t.Fatalf("render trash: %v", err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`<strong>2</strong>`,
		`action="/messages/restore/srv_message"`,
		`action="/contacts/srv_contact/restore"`,
		`name="csrf_token" value="TOKEN-123"`,
		`aria-label="Restore contact Alice"`,
		`hx-trigger="refresh from:body, htmx:oobAfterSwap from:body"`,
		`href="/messages" class="nav-link"`,
		`href="/messages/trash" class="nav-link active"`,
		`&lt;script&gt;alert(1)&lt;/script&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("rendered body contains unescaped message author")
	}
}
