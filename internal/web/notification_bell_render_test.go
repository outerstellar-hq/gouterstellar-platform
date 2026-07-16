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

func TestNotificationBellRendersUnreadStateAndCapsVisualCount(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	tests := []struct {
		name    string
		count   int64
		present []string
		absent  string
	}{
		{"empty", 0, []string{`aria-label="Notifications"`, `href="/notifications"`}, `notification-bell-count`},
		{"unread", 120, []string{`aria-label="Notifications, 120 unread"`, `class="notification-bell-count"`, `>99+</span>`}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			if err := renderer.RenderPartial(recorder, "notification_bell", viewmodel.NotificationBell{UnreadCount: test.count}); err != nil {
				t.Fatalf("render bell: %v", err)
			}
			body := recorder.Body.String()
			for _, want := range test.present {
				if !strings.Contains(body, want) {
					t.Errorf("rendered bell missing %q", want)
				}
			}
			if test.absent != "" && strings.Contains(body, test.absent) {
				t.Errorf("rendered bell unexpectedly contains %q", test.absent)
			}
		})
	}
}

func TestAuthenticatedShellLoadsBellWithUsableFallback(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	if err != nil {
		t.Fatalf("build renderer: %v", err)
	}

	req := web.WithUser(httptest.NewRequest(http.MethodGet, "/", nil), &model.User{
		ID: uuid.New(), Username: "alex", Role: model.RoleUser,
	})
	recorder := httptest.NewRecorder()
	if err := renderer.RenderPage(recorder, req, "messages", viewmodel.MessagesPage{}); err != nil {
		t.Fatalf("render shell: %v", err)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`hx-boost="true" hx-ext="ws" ws-connect="/ws/sync"`,
		`id="ws-updates" ws-subscribe aria-live="polite"`,
		`id="notification-bell"`,
		`hx-get="/components/notification-bell?lang=en"`,
		`hx-trigger="load, every 60s"`,
		`class="notification-bell notification-bell-fallback">Notifications</a>`,
		`src="/static/js/htmx-ext-ws.js"`,
		`action="/logout" hx-boost="false"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered shell missing %q", want)
		}
	}
}
