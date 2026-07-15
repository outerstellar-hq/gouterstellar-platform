package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

func TestRendererProducesCompleteNonEmptyPage(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test-version")
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = WithUser(request, &model.User{ID: uuid.New(), Username: "admin", Role: model.RoleAdmin})
	request = WithCSRFToken(request, "csrf-token")
	require.NoError(t, renderer.Render(response, request, "home.html", viewmodel.HomePage{}))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Welcome to Outerstellar Platform")
	assert.Contains(t, response.Body.String(), `href="/messages"`)
	assert.Contains(t, response.Body.String(), `href="/messages/trash"`)
	assert.Contains(t, response.Body.String(), `content="csrf-token"`)
	assert.Contains(t, response.Body.String(), "admin")
	assert.Contains(t, response.Body.String(), "vtest-version")
	assert.Contains(t, response.Body.String(), "/static/js/platform.js")
}

func TestRendererRejectsMissingTemplate(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test-version")
	require.NoError(t, err)
	err = renderer.Render(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil), "missing.html", nil)
	assert.ErrorContains(t, err, "does not exist")
}

func TestRendererPreservesErrorStatus(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test-version")
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	require.NoError(t, renderer.RenderWithStatus(response, request, "error.html", viewmodel.ErrorPage{StatusCode: 404, Title: "Missing"}, http.StatusNotFound))
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Body.String(), "404 - Missing")
}

func TestRendererProducesContentOnlyForComponent(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test-version")
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/components/message-list", nil)
	require.NoError(t, renderer.Render(response, request, "components/message_list.html", viewmodel.MessagesPage{}))
	assert.NotContains(t, response.Body.String(), "<!DOCTYPE html>")
	assert.Contains(t, response.Body.String(), "No messages found")
}

func TestRendererUsesWorkingAdminAndMessageActions(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test-version")
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	request = WithUser(request, &model.User{ID: uuid.New(), Username: "admin", Role: model.RoleAdmin})

	adminResponse := httptest.NewRecorder()
	require.NoError(t, renderer.Render(adminResponse, request, "admin_users.html", viewmodel.AdminUsersPage{Users: []viewmodel.UserItem{
		{ID: "user-id", Username: "alice", Enabled: true},
	}}))
	assert.Contains(t, adminResponse.Body.String(), `action="/admin/users/user-id/enabled"`)
	assert.Contains(t, adminResponse.Body.String(), `name="enabled" value="false"`)

	messageResponse := httptest.NewRecorder()
	require.NoError(t, renderer.Render(messageResponse, request, "messages.html", viewmodel.MessagesPage{Messages: []viewmodel.MessageItem{
		{SyncID: "message-id", Author: "Alice", Content: "Hello"},
	}}))
	assert.Contains(t, messageResponse.Body.String(), `action="/messages"`)
	assert.Contains(t, messageResponse.Body.String(), `action="/messages/message-id/delete"`)
}
