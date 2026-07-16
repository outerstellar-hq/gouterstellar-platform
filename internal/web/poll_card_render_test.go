package web_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

func TestPollCardRendersSelectedAndBlockedSingleChoiceStates(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}
	recorder := httptest.NewRecorder()
	err = renderer.RenderPartial(recorder, "poll_card", viewmodel.PollCard{
		SyncID: "poll-1", Question: "Choose a color", TotalVotes: 3, CSRFToken: "TOKEN-123",
		Options: []viewmodel.PollOption{
			{ID: 10, Text: "Red", VoteCount: 2, Percent: 66, Selected: true},
			{ID: 11, Text: "Blue", VoteCount: 1, Percent: 33},
		},
	})
	if err != nil {
		t.Fatalf("render poll card: %v", err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`aria-label="Red: 66 percent, 2 votes"`,
		`action="/components/polls/poll-1/remove-vote"`,
		`name="csrf_token" value="TOKEN-123"`,
		`aria-pressed="true">Remove vote`,
		`Remove your current vote to choose another option.`,
		`3 total votes`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
}

func TestClosedPollCardHidesMutationControls(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}
	recorder := httptest.NewRecorder()
	err = renderer.RenderPartial(recorder, "poll_card", viewmodel.PollCard{
		SyncID: "poll-1", Question: "Closed poll", Closed: true,
		Options: []viewmodel.PollOption{{ID: 10, Text: "Done"}},
	})
	if err != nil {
		t.Fatalf("render poll card: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Closed") || strings.Contains(body, "poll-vote-form") {
		t.Errorf("closed poll did not render as immutable: %s", body)
	}
}
