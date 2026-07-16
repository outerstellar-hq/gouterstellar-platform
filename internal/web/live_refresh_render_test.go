package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

func TestMessageAndContactPagesExposeLiveRefreshFragments(t *testing.T) {
	renderer, err := web.NewRenderer(web.TemplateFS(), web.TemplateFuncMap(), "test")
	require.NoError(t, err)
	user := &model.User{ID: uuid.New(), Username: "alex", Role: model.RoleUser}

	messagesRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/?q=hello", nil), user)
	messagesRecorder := httptest.NewRecorder()
	require.NoError(t, renderer.RenderPage(messagesRecorder, messagesRequest, "messages", viewmodel.MessagesPage{
		Messages:   []viewmodel.MessageItem{{SyncID: "srv_message", Author: "Alice", Content: "Hello", Language: "en"}},
		RefreshURL: "/components/message-list?q=hello&limit=10&offset=0",
	}))
	assert.Contains(t, messagesRecorder.Body.String(), `id="message-list-container"`)
	assert.Contains(t, messagesRecorder.Body.String(), `hx-get="/components/message-list?q=hello&amp;limit=10&amp;offset=0"`)
	assert.Contains(t, messagesRecorder.Body.String(), `hx-trigger="refresh from:body, htmx:oobAfterSwap from:body"`)
	assert.Contains(t, messagesRecorder.Body.String(), `/messages/srv_message/edit`)

	contactsRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/contacts?q=alice", nil), user)
	contactsRecorder := httptest.NewRecorder()
	require.NoError(t, renderer.RenderPage(contactsRecorder, contactsRequest, "contacts", viewmodel.ContactsPage{
		Contacts:   []viewmodel.ContactItem{{SyncID: "srv_contact", Name: "Alice", Language: "en"}},
		RefreshURL: "/components/contact-list?q=alice&limit=12&offset=0",
	}))
	assert.Contains(t, contactsRecorder.Body.String(), `id="contact-list-container"`)
	assert.Contains(t, contactsRecorder.Body.String(), `hx-get="/components/contact-list?q=alice&amp;limit=12&amp;offset=0"`)
	assert.Contains(t, contactsRecorder.Body.String(), `hx-trigger="refresh from:body, htmx:oobAfterSwap from:body"`)
	assert.Contains(t, contactsRecorder.Body.String(), `/contacts/srv_contact/edit`)
}
