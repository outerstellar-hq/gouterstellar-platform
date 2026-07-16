package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

func TestMessageEditPageRendersSafeAccessibleForm(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	req := web.WithCSRFToken(httptest.NewRequest(http.MethodGet, "/messages/srv_abc/edit", nil), "TOKEN-123")
	recorder := httptest.NewRecorder()
	err = renderer.RenderPage(recorder, req, "message_edit", viewmodel.MessageEditPage{
		SyncID: "srv_abc", Author: `Alice "Admin"`, Content: "<script>alert(1)</script>", Error: "Author and content are required.",
	})
	if err != nil {
		t.Fatalf("render edit page: %v", err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`action="/messages/srv_abc/update"`,
		`name="csrf_token" value="TOKEN-123"`,
		`<label for="message-edit-author">Author</label>`,
		`<label for="message-edit-content">Content</label>`,
		`role="alert">Author and content are required.</div>`,
		`>Save</button>`,
		`&lt;script&gt;alert(1)&lt;/script&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("rendered body contains unescaped message content")
	}
}

func TestMessagesPageExposesEditControl(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	recorder := httptest.NewRecorder()
	err = renderer.RenderPage(recorder, httptest.NewRequest(http.MethodGet, "/messages", nil), "messages", viewmodel.MessagesPage{
		Messages: []viewmodel.MessageItem{{SyncID: "srv_abc", Author: "Alice", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("render messages page: %v", err)
	}

	if !strings.Contains(recorder.Body.String(), `href="/messages/srv_abc/edit"`) {
		t.Error("messages page does not expose the edit route")
	}
}
