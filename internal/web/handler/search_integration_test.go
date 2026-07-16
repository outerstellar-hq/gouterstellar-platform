package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/config"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/platform/core"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/filter"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/wire"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
	"github.com/outerstellar-hq/gouterstellar-platform/platform/migration"
)

func TestUnifiedSearchUsesRealMessageAndContactData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database-backed search test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connectionString, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.Eventually(t, func() bool { return pool.Ping(ctx) == nil }, 30*time.Second, 500*time.Millisecond)

	coreExtension := core.NewExtension()
	require.NoError(t, migration.NewRunner(pool, coreExtension.Manifest().Migrations).Run(ctx))

	cfg := config.Load()
	cfg.DatabaseURL = connectionString
	cfg.Version = "search-test"
	cfg.OAuth.Apple.Enabled = true
	cfg.OAuth.Apple.ClientID = "test-client"
	app := wire.Wire(cfg, pool, web.TemplateFS())

	_, err = app.MessageService.CreateServerMessage(ctx, "Ada", "Project Aurora launch notes")
	require.NoError(t, err)

	contact, err := app.ContactService.CreateContact(
		ctx,
		"Aurora Adams",
		[]string{"aurora@example.com"},
		nil,
		nil,
		"Outerstellar",
		"",
		"Research",
	)
	require.NoError(t, err)

	user, err := app.SecurityService.Register(ctx, "searcher", "SearchPass1!")
	require.NoError(t, err)
	catalog := extplatform.NewCatalog()
	webExtension := wire.BuildCoreExtension(app, catalog)
	stub := http.NotFoundHandler()
	webExtension.SetOperations(stub, stub, stub, stub)
	webExtension.SetDiagnostics(stub)
	webExtension.SetMetrics(stub)
	webExtension.SetStatic(stub)
	assembled, err := extplatform.NewHandler(extplatform.Options{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{webExtension},
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			extplatform.GroupProtectedUI: {filter.RequireAuthenticated},
		},
		NotFoundHandler: http.HandlerFunc(app.ErrorHandler.NotFound),
		Catalog:         catalog,
	})
	require.NoError(t, err)

	loginResponse := httptest.NewRecorder()
	assembled.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodGet, "/auth", nil))
	require.Equal(t, http.StatusOK, loginResponse.Code)
	assert.Contains(t, loginResponse.Body.String(), "Sign in with Apple")

	anonymousBell := httptest.NewRecorder()
	assembled.ServeHTTP(anonymousBell, httptest.NewRequest(http.MethodGet, "/components/notification-bell", nil))
	require.Equal(t, http.StatusOK, anonymousBell.Code)
	assert.Contains(t, anonymousBell.Body.String(), "Notifications")

	authForm := httptest.NewRecorder()
	assembled.ServeHTTP(authForm, httptest.NewRequest(http.MethodGet, "/auth/components/forms/register?returnTo=%2Fcontacts", nil))
	require.Equal(t, http.StatusOK, authForm.Code)
	assert.Contains(t, authForm.Body.String(), `data-auth-mode="register"`)
	assert.Contains(t, authForm.Body.String(), `name="returnTo" value="/contacts"`)
	assert.NotContains(t, authForm.Body.String(), "<html")

	resetMismatch := httptest.NewRecorder()
	resetForm := url.Values{
		"token":           {"invalid-token"},
		"newPassword":     {"ValidPassword1!"},
		"confirmPassword": {"different"},
	}
	resetRequest := httptest.NewRequest(http.MethodPost, "/auth/components/reset-confirm", strings.NewReader(resetForm.Encode()))
	resetRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	assembled.ServeHTTP(resetMismatch, resetRequest)
	require.Equal(t, http.StatusOK, resetMismatch.Code)
	assert.Contains(t, resetMismatch.Body.String(), "Password reset failed")
	assert.Contains(t, resetMismatch.Body.String(), "do not match")

	errorPage := httptest.NewRecorder()
	assembled.ServeHTTP(errorPage, httptest.NewRequest(http.MethodGet, "/errors/not-found", nil))
	require.Equal(t, http.StatusOK, errorPage.Code)
	assert.Contains(t, errorPage.Body.String(), "The page could not be found")

	errorHelp := httptest.NewRecorder()
	assembled.ServeHTTP(errorHelp, httptest.NewRequest(http.MethodGet, "/errors/components/help/not-found", nil))
	require.Equal(t, http.StatusOK, errorHelp.Code)
	assert.Contains(t, errorHelp.Body.String(), "Not found recovery tips")

	missingPage := httptest.NewRecorder()
	assembled.ServeHTTP(missingPage, httptest.NewRequest(http.MethodGet, "/definitely-missing", nil))
	require.Equal(t, http.StatusNotFound, missingPage.Code)
	assert.Contains(t, missingPage.Body.String(), "sidebar navigation and recovery actions")
	assert.Equal(t, "text/html; charset=utf-8", missingPage.Header().Get("Content-Type"))

	publicOpenAPI := httptest.NewRecorder()
	assembled.ServeHTTP(publicOpenAPI, httptest.NewRequest(http.MethodGet, "/ui/openapi.json", nil))
	require.Equal(t, http.StatusOK, publicOpenAPI.Code)
	assert.Contains(t, publicOpenAPI.Body.String(), `"openapi":"3.0.3"`)

	protectedOpenAPI := httptest.NewRecorder()
	assembled.ServeHTTP(protectedOpenAPI, httptest.NewRequest(http.MethodGet, "/ui-protected/openapi.json", nil))
	require.Equal(t, http.StatusSeeOther, protectedOpenAPI.Code)

	totpStatus := httptest.NewRecorder()
	assembled.ServeHTTP(totpStatus, web.WithUser(httptest.NewRequest(http.MethodGet, "/auth/components/totp-setup-status", nil), user))
	require.Equal(t, http.StatusOK, totpStatus.Code)
	assert.Contains(t, totpStatus.Body.String(), "Enable Two-Factor Auth")

	totpSetup := httptest.NewRecorder()
	totpSetupRequest := web.WithUser(httptest.NewRequest(http.MethodPost, "/auth/components/totp-setup", nil), user)
	assembled.ServeHTTP(totpSetup, totpSetupRequest)
	require.Equal(t, http.StatusOK, totpSetup.Code)
	assert.Contains(t, totpSetup.Body.String(), "Or enter this key manually")
	assert.Contains(t, totpSetup.Body.String(), "data:image/png;base64")

	profileUpdate := httptest.NewRecorder()
	profileForm := url.Values{"email": {"searcher@example.com"}, "username": {"searcher"}}
	profileRequest := httptest.NewRequest(http.MethodPost, "/auth/components/profile-update", strings.NewReader(profileForm.Encode()))
	profileRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	assembled.ServeHTTP(profileUpdate, web.WithUser(profileRequest, user))
	require.Equal(t, http.StatusOK, profileUpdate.Code)
	assert.Contains(t, profileUpdate.Body.String(), "Profile updated")
	assert.NotContains(t, profileUpdate.Body.String(), "<html")

	notificationUpdate := httptest.NewRecorder()
	notificationForm := url.Values{"emailNotifications": {"on"}, "pushNotifications": {"on"}}
	notificationRequest := httptest.NewRequest(http.MethodPost, "/auth/notification-preferences", strings.NewReader(notificationForm.Encode()))
	notificationRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	assembled.ServeHTTP(notificationUpdate, web.WithUser(notificationRequest, user))
	require.Equal(t, http.StatusOK, notificationUpdate.Code)
	assert.Contains(t, notificationUpdate.Body.String(), "Preferences updated")

	appleStart := httptest.NewRecorder()
	assembled.ServeHTTP(appleStart, httptest.NewRequest(http.MethodGet, "/auth/oauth/apple", nil))
	require.Equal(t, http.StatusFound, appleStart.Code)
	assert.Equal(t, "/auth/oauth/apple/not-configured", appleStart.Header().Get("Location"))
	assert.Contains(t, appleStart.Header().Get("Set-Cookie"), "oauth_state=")

	appleUnavailable := httptest.NewRecorder()
	assembled.ServeHTTP(appleUnavailable, httptest.NewRequest(http.MethodGet, "/auth/oauth/apple/not-configured", nil))
	require.Equal(t, http.StatusServiceUnavailable, appleUnavailable.Code)
	assert.Contains(t, appleUnavailable.Body.String(), "not yet configured")

	homeRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/?q=Aurora&limit=10&offset=0&theme=dracula&lang=fr&layout=compact", nil), user)
	homeResponse := httptest.NewRecorder()
	assembled.ServeHTTP(homeResponse, homeRequest)
	require.Equal(t, http.StatusOK, homeResponse.Code)
	assert.Contains(t, homeResponse.Body.String(), "Project Aurora launch notes")
	assert.Contains(t, homeResponse.Body.String(), `action="/messages"`)
	assert.Contains(t, homeResponse.Body.String(), `<html lang="fr" data-theme="dracula">`)
	assert.Contains(t, homeResponse.Body.String(), "layout-sidebar density-compact")
	assert.Contains(t, homeResponse.Body.String(), "Se déconnecter")
	assert.NotContains(t, homeResponse.Body.String(), "Welcome to Outerstellar Platform")

	messageList := httptest.NewRecorder()
	messageListRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/components/message-list?q=Aurora&limit=1&offset=0", nil), user)
	assembled.ServeHTTP(messageList, messageListRequest)
	require.Equal(t, http.StatusOK, messageList.Code)
	assert.Contains(t, messageList.Body.String(), "Project Aurora launch notes")

	footerStatus := httptest.NewRecorder()
	assembled.ServeHTTP(footerStatus, httptest.NewRequest(http.MethodGet, "/components/footer-status", nil))
	require.Equal(t, http.StatusOK, footerStatus.Code)
	assert.Contains(t, footerStatus.Body.String(), "Server messages: 1")
	assert.Contains(t, footerStatus.Body.String(), "Pending sync messages: 0")

	themeSelector := httptest.NewRecorder()
	assembled.ServeHTTP(themeSelector, web.WithUser(httptest.NewRequest(http.MethodGet,
		"/components/sidebar/theme-selector?pagePath=%2Fcontacts&theme=dracula&lang=fr&layout=compact", nil), user))
	require.Equal(t, http.StatusOK, themeSelector.Code)
	assert.Contains(t, themeSelector.Body.String(), `value="dracula" selected`)
	assert.Contains(t, themeSelector.Body.String(), `name="pagePath" value="/contacts"`)

	navigation := httptest.NewRecorder()
	assembled.ServeHTTP(navigation, httptest.NewRequest(http.MethodGet,
		"/components/navigation/page?pagePath=%2Fcontacts&theme=dark&lang=fr&layout=compact", nil))
	require.Equal(t, http.StatusOK, navigation.Code)
	assert.Equal(t, "/contacts?lang=fr&layout=compact&theme=dark", navigation.Header().Get("HX-Redirect"))

	htmlRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/search?q=Aurora", nil), user)
	htmlResponse := httptest.NewRecorder()
	app.SearchHandler.Search(htmlResponse, htmlRequest)

	require.Equal(t, http.StatusOK, htmlResponse.Code)
	body := htmlResponse.Body.String()
	assert.Contains(t, body, "Project Aurora launch notes")
	assert.Contains(t, body, "Aurora Adams")
	assert.Contains(t, body, "search-result-message")
	assert.Contains(t, body, "search-result-contact")
	assert.Contains(t, body, "/contacts/"+contact.SyncID)

	filteredRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/search?q=Aurora&type=contact", nil), user)
	filteredResponse := httptest.NewRecorder()
	app.SearchHandler.Search(filteredResponse, filteredRequest)
	require.Equal(t, http.StatusOK, filteredResponse.Code)
	assert.Contains(t, filteredResponse.Body.String(), "Aurora Adams")
	assert.NotContains(t, filteredResponse.Body.String(), "Project Aurora launch notes")
	assert.Contains(t, filteredResponse.Body.String(), "1 result for")

	apiRequest := web.WithUser(httptest.NewRequest(http.MethodGet, "/api/v1/search?q=Aurora", nil), user)
	apiResponse := httptest.NewRecorder()
	app.SearchHandler.SearchAPI(apiResponse, apiRequest)
	require.Equal(t, http.StatusOK, apiResponse.Code)

	var payload struct {
		Query   string                   `json:"query"`
		Results []viewmodel.SearchResult `json:"results"`
		Total   int                      `json:"total"`
	}
	require.NoError(t, json.NewDecoder(apiResponse.Body).Decode(&payload))
	assert.Equal(t, "Aurora", payload.Query)
	assert.Equal(t, 2, payload.Total)
	assert.Len(t, payload.Results, 2)

	unauthenticated := httptest.NewRecorder()
	app.SearchHandler.SearchAPI(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=Aurora", nil))
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	unauthenticatedBody, err := io.ReadAll(unauthenticated.Body)
	require.NoError(t, err)
	assert.Contains(t, string(unauthenticatedBody), "Authentication required")

	newContact := httptest.NewRecorder()
	assembled.ServeHTTP(newContact, web.WithUser(httptest.NewRequest(http.MethodGet, "/contacts/new", nil), user))
	require.Equal(t, http.StatusOK, newContact.Code)
	assert.Contains(t, newContact.Body.String(), "<html")
	assert.Contains(t, newContact.Body.String(), "Create Contact")
	assert.Contains(t, newContact.Body.String(), `action="/contacts"`)
	assert.Contains(t, newContact.Body.String(), `name="socialMedia"`)

	editContact := httptest.NewRecorder()
	assembled.ServeHTTP(editContact, web.WithUser(httptest.NewRequest(http.MethodGet, "/contacts/"+contact.SyncID+"/edit", nil), user))
	require.Equal(t, http.StatusOK, editContact.Code)
	assert.Contains(t, editContact.Body.String(), "<html")
	assert.Contains(t, editContact.Body.String(), "Edit contact")
	assert.Contains(t, editContact.Body.String(), "Aurora Adams")
	assert.Contains(t, editContact.Body.String(), "/contacts/"+contact.SyncID+"/update")

	require.NoError(t, app.ContactService.DeleteContact(ctx, contact.SyncID))
	trashContacts := httptest.NewRecorder()
	assembled.ServeHTTP(trashContacts, web.WithUser(httptest.NewRequest(http.MethodGet, "/contacts/trash/list", nil), user))
	require.Equal(t, http.StatusOK, trashContacts.Code)
	assert.Contains(t, trashContacts.Body.String(), "Aurora Adams")
	assert.Contains(t, trashContacts.Body.String(), "/contacts/"+contact.SyncID+"/restore")

	deleteUser, err := app.SecurityService.Register(ctx, "delete-me", "DeleteMe-2026!")
	require.NoError(t, err)
	deleteForm := url.Values{"currentPassword": []string{"DeleteMe-2026!"}}
	deleteRequest := httptest.NewRequest(http.MethodPost, "/auth/account/delete", strings.NewReader(deleteForm.Encode()))
	deleteRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	deleteResponse := httptest.NewRecorder()
	assembled.ServeHTTP(deleteResponse, web.WithUser(deleteRequest, deleteUser))
	require.Equal(t, http.StatusFound, deleteResponse.Code)
	assert.Equal(t, "/auth?deleted=true", deleteResponse.Header().Get("Location"))
	assert.Contains(t, deleteResponse.Header().Get("Set-Cookie"), web.SessionCookieName+"=")
	assert.Contains(t, deleteResponse.Header().Get("Set-Cookie"), "Max-Age=0")
	_, authErr := app.SecurityService.Authenticate(ctx, "delete-me", "DeleteMe-2026!")
	assert.Error(t, authErr)
}
