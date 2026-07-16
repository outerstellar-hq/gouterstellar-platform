package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

func TestMessagesPageRendersAccessibleVotingControls(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}
	req := web.WithCSRFToken(httptest.NewRequest(http.MethodGet, "/messages", nil), "TOKEN-123")
	recorder := httptest.NewRecorder()

	err = renderer.RenderPage(recorder, req, "messages", viewmodel.MessagesPage{Messages: []viewmodel.MessageItem{{
		SyncID: "srv_abc", Author: "Alice", Content: "Hello", NetScore: 2, Upvotes: 3,
		Downvotes: 1, HasUpvoted: true, CSRFToken: "TOKEN-123",
	}}})
	if err != nil {
		t.Fatalf("render messages: %v", err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`hx-post="/components/messages/srv_abc/vote"`,
		`name="csrf_token" value="TOKEN-123"`,
		`aria-label="Upvote. Current upvotes: 3"`,
		`aria-pressed="true"`,
		`aria-label="Downvote. Current downvotes: 1"`,
		`<output class="vote-score" aria-live="polite" title="Net score">2</output>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
}
