package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/config"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/platform/core"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/security"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
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
	cfg.DevDashboardEnabled = true
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
	webExtension.SetStatic(fstest.MapFS{
		"css/main.css": &fstest.MapFile{Data: []byte("body {}")},
		"swagger.html": &fstest.MapFile{Data: []byte("<html></html>")},
	})
	assembled, err := extplatform.NewHandler(extplatform.Options{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{webExtension},
		MiddlewareChain: []func(http.Handler) http.Handler{
			filter.Logging(),
			filter.CORS("*"),
			filter.SecurityHeaders(cfg.CSPPolicy, false),
			filter.Session(app.SecurityService, false),
		},
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			extplatform.GroupProtectedUI: {filter.RequireAuthenticated},
			extplatform.GroupAPI:         {filter.BearerAuth(app.AuthMetrics, app.Realms...)},
			extplatform.GroupAdmin:       {filter.RequirePermission(app.PermissionResolver, "*", "*")},
		},
		NotFoundHandler: http.HandlerFunc(app.ErrorHandler.NotFound),
		Catalog:         catalog,
	})
	require.NoError(t, err)

	expiredToken, err := app.SecurityService.CreateSession(ctx, user.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE plt_sessions SET expires_at = CURRENT_TIMESTAMP - INTERVAL '1 hour' WHERE user_id = $1", user.ID)
	require.NoError(t, err)
	activeToken, err := app.SecurityService.CreateSession(ctx, user.ID)
	require.NoError(t, err)

	expiredBrowserRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	expiredBrowserRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: expiredToken})
	expiredBrowser := httptest.NewRecorder()
	assembled.ServeHTTP(expiredBrowser, expiredBrowserRequest)
	require.Equal(t, http.StatusFound, expiredBrowser.Code)
	assert.Equal(t, "/auth?expired=true", expiredBrowser.Header().Get("Location"))
	assert.Equal(t, "true", expiredBrowser.Header().Get(filter.SessionExpiredHeader))
	assert.Contains(t, expiredBrowser.Header().Get("Set-Cookie"), "Max-Age=0")

	expiredAPIRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	expiredAPIRequest.Header.Set("Authorization", "Bearer "+expiredToken)
	expiredAPI := httptest.NewRecorder()
	assembled.ServeHTTP(expiredAPI, expiredAPIRequest)
	require.Equal(t, http.StatusUnauthorized, expiredAPI.Code)
	assert.Equal(t, "true", expiredAPI.Header().Get(filter.SessionExpiredHeader))
	assert.Contains(t, expiredAPI.Body.String(), "expired")
	assert.Empty(t, expiredAPI.Header().Get("Content-Security-Policy"))
	assert.Contains(t, expiredAPI.Header().Get("Access-Control-Expose-Headers"), filter.SessionExpiredHeader)

	activeBrowserRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	activeBrowserRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: activeToken})
	activeBrowser := httptest.NewRecorder()
	assembled.ServeHTTP(activeBrowser, activeBrowserRequest)
	require.Equal(t, http.StatusOK, activeBrowser.Code)
	assert.Contains(t, activeBrowser.Header().Get("Set-Cookie"), web.SessionCookieName+"="+activeToken)

	internalLoginForm := url.Values{
		"username": {"searcher"},
		"password": {"SearchPass1!"},
		"returnTo": {"/contacts"},
	}
	internalLogin := httptest.NewRecorder()
	internalLoginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(internalLoginForm.Encode()))
	internalLoginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.AuthHandler.HandleLogin(internalLogin, internalLoginRequest)
	require.Equal(t, http.StatusFound, internalLogin.Code)
	assert.Equal(t, "/contacts", internalLogin.Header().Get("Location"))

	externalLoginForm := url.Values{
		"username": {"searcher"},
		"password": {"SearchPass1!"},
		"returnTo": {"https://example.invalid/steal"},
	}
	externalLogin := httptest.NewRecorder()
	externalLoginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(externalLoginForm.Encode()))
	externalLoginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.AuthHandler.HandleLogin(externalLogin, externalLoginRequest)
	require.Equal(t, http.StatusFound, externalLogin.Code)
	assert.Equal(t, "/", externalLogin.Header().Get("Location"))

	resetSvc := service.NewPasswordResetService(
		persistence.NewUserRepository(pool),
		security.NewBCryptPasswordEncoder(12),
		persistence.NewPasswordResetRepository(pool),
		&service.NoOpEmailService{},
		service.NewAuditService(persistence.NewAuditRepository(pool)),
		cfg.AppBaseURL,
		cfg.TokenPepper,
	)
	firstResetToken, err := resetSvc.RequestPasswordReset(ctx, user.Email)
	require.NoError(t, err)
	require.NotNil(t, firstResetToken)
	assert.True(t, strings.HasPrefix(*firstResetToken, "prt_"))
	var storedTokenHash string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT token FROM plt_password_reset_tokens WHERE user_id = $1 ORDER BY id DESC LIMIT 1", user.ID,
	).Scan(&storedTokenHash))
	assert.NotEqual(t, *firstResetToken, storedTokenHash)
	assert.Equal(t, 44, len(storedTokenHash))
	assert.Equal(t, security.NewTokenHasher(cfg.TokenPepper).Hash(*firstResetToken), storedTokenHash)

	secondResetToken, err := resetSvc.RequestPasswordReset(ctx, user.Email)
	require.NoError(t, err)
	require.NotNil(t, secondResetToken)
	assert.NotEqual(t, *firstResetToken, *secondResetToken)
	require.Error(t, resetSvc.ResetPassword(ctx, *firstResetToken, "ReplacementPass2!"))

	resetTokenPage := httptest.NewRecorder()
	assembled.ServeHTTP(resetTokenPage, httptest.NewRequest(http.MethodGet, "/auth/reset/"+*secondResetToken, nil))
	require.Equal(t, http.StatusOK, resetTokenPage.Code)
	assert.Contains(t, resetTokenPage.Body.String(), *secondResetToken)

	confirmBody := strings.NewReader(`{"token":"` + *secondResetToken + `","newPassword":"ReplacementPass2!"}`)
	confirmReset := httptest.NewRecorder()
	app.AuthAPI.ConfirmPasswordReset(confirmReset, httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-confirm", confirmBody))
	require.Equal(t, http.StatusOK, confirmReset.Code)

	var remainingSessions int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_sessions WHERE user_id = $1", user.ID).Scan(&remainingSessions))
	assert.Zero(t, remainingSessions)
	_, err = app.SecurityService.Authenticate(ctx, "searcher", "SearchPass1!")
	assert.Error(t, err)
	_, err = app.SecurityService.Authenticate(ctx, "searcher", "ReplacementPass2!")
	require.NoError(t, err)

	replayBody := strings.NewReader(`{"token":"` + *secondResetToken + `","newPassword":"AnotherReplacement3!"}`)
	replayReset := httptest.NewRecorder()
	app.AuthAPI.ConfirmPasswordReset(replayReset, httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-confirm", replayBody))
	require.Equal(t, http.StatusBadRequest, replayReset.Code)

	oauthSvc := security.NewOAuthService(
		persistence.NewUserRepository(pool),
		persistence.NewOAuthRepository(pool),
		security.NewBCryptPasswordEncoder(12),
	)
	oauthEmail := "searcher@oauth.example"
	oauthUser, err := oauthSvc.FindOrCreateOAuthUser(ctx, "apple", "apple.subject.searcher", &oauthEmail)
	require.NoError(t, err)
	assert.Equal(t, "searcher2", oauthUser.Username)
	repeatedOAuthUser, err := oauthSvc.FindOrCreateOAuthUser(ctx, "apple", "apple.subject.searcher", &oauthEmail)
	require.NoError(t, err)
	assert.Equal(t, oauthUser.ID, repeatedOAuthUser.ID)
	var oauthConnectionCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM plt_oauth_connections WHERE user_id = $1 AND provider = 'apple'", oauthUser.ID,
	).Scan(&oauthConnectionCount))
	assert.Equal(t, 1, oauthConnectionCount)
	providerNamedUser, err := oauthSvc.FindOrCreateOAuthUser(ctx, "apple", "apple.subject.no-email", nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(providerNamedUser.Username, "apple_"))

	correlatedRequest := httptest.NewRequest(http.MethodGet, "/auth", nil)
	correlatedRequest.Header.Set(filter.RequestIDHeader, "feature-session-request-42")
	correlated := httptest.NewRecorder()
	assembled.ServeHTTP(correlated, correlatedRequest)
	require.Equal(t, http.StatusOK, correlated.Code)
	assert.Equal(t, "feature-session-request-42", correlated.Header().Get(filter.RequestIDHeader))
	correlatedCSP := correlated.Header().Get("Content-Security-Policy")
	assert.Contains(t, correlatedCSP, "default-src")
	nonceStart := strings.Index(correlatedCSP, "'nonce-")
	require.NotEqual(t, -1, nonceStart)
	nonceRemainder := correlatedCSP[nonceStart+len("'nonce-"):]
	nonceEnd := strings.Index(nonceRemainder, "'")
	require.NotEqual(t, -1, nonceEnd)
	assert.Contains(t, correlated.Body.String(), `nonce="`+nonceRemainder[:nonceEnd]+`"`)
	assert.Contains(t, correlated.Header().Get("Access-Control-Expose-Headers"), filter.RequestIDHeader)

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
	require.Equal(t, http.StatusFound, protectedOpenAPI.Code)
	protectedLocation, err := url.Parse(protectedOpenAPI.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/ui-protected/openapi.json", protectedLocation.Query().Get("returnTo"))

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
	assert.Contains(t, appleStart.Header().Get("Set-Cookie"), "HttpOnly")
	assert.Contains(t, appleStart.Header().Get("Set-Cookie"), "Max-Age=600")

	unknownOAuth := httptest.NewRecorder()
	assembled.ServeHTTP(unknownOAuth, httptest.NewRequest(http.MethodGet, "/auth/oauth/unknown", nil))
	require.Equal(t, http.StatusNotFound, unknownOAuth.Code)

	missingOAuthState := httptest.NewRecorder()
	assembled.ServeHTTP(missingOAuthState, httptest.NewRequest(http.MethodGet, "/auth/oauth/apple/callback?code=test&state=missing", nil))
	require.Equal(t, http.StatusFound, missingOAuthState.Code)
	assert.Equal(t, "/auth?oauth_error=true", missingOAuthState.Header().Get("Location"))

	mismatchedOAuthRequest := httptest.NewRequest(http.MethodGet, "/auth/oauth/apple/callback?code=test&state=wrong", nil)
	mismatchedOAuthRequest.AddCookie(&http.Cookie{Name: "oauth_state", Value: "correct"})
	mismatchedOAuth := httptest.NewRecorder()
	assembled.ServeHTTP(mismatchedOAuth, mismatchedOAuthRequest)
	require.Equal(t, http.StatusFound, mismatchedOAuth.Code)
	assert.Equal(t, "/auth?oauth_error=true", mismatchedOAuth.Header().Get("Location"))

	providerFailureRequest := httptest.NewRequest(http.MethodGet, "/auth/oauth/apple/callback?code=test&state=matching", nil)
	providerFailureRequest.AddCookie(&http.Cookie{Name: "oauth_state", Value: "matching"})
	providerFailure := httptest.NewRecorder()
	assembled.ServeHTTP(providerFailure, providerFailureRequest)
	require.Equal(t, http.StatusFound, providerFailure.Code)
	assert.Equal(t, "/auth?oauth_error=true", providerFailure.Header().Get("Location"))

	malformedOAuthRequest := httptest.NewRequest(http.MethodPost, "/auth/oauth/apple/callback", strings.NewReader("code=%ZZ&state=matching"))
	malformedOAuthRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	malformedOAuthRequest.AddCookie(&http.Cookie{Name: "oauth_state", Value: "matching"})
	malformedOAuth := httptest.NewRecorder()
	assembled.ServeHTTP(malformedOAuth, malformedOAuthRequest)
	require.Equal(t, http.StatusFound, malformedOAuth.Code)
	assert.Equal(t, "/auth?oauth_error=true", malformedOAuth.Header().Get("Location"))

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

	syncToken, err := app.SecurityService.CreateSession(ctx, user.ID)
	require.NoError(t, err)
	conflictSyncID := "sync-conflict-" + user.ID.String()[:16]
	pushSync := func(content string, timestamp int64) *httptest.ResponseRecorder {
		t.Helper()
		body := strings.NewReader(`{"messages":[{"syncId":"` + conflictSyncID + `","author":"searcher","content":"` + content + `","updatedAtEpochMs":` + strconv.FormatInt(timestamp, 10) + `}]}`)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/sync", body)
		request.Header.Set("Authorization", "Bearer "+syncToken)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)
		return response
	}

	initialSync := pushSync("Server version", 2000)
	require.Equal(t, http.StatusOK, initialSync.Code)
	staleSync := pushSync("Client version", 1000)
	require.Equal(t, http.StatusOK, staleSync.Code)
	var syncResult model.SyncPushResponse
	require.NoError(t, json.NewDecoder(staleSync.Body).Decode(&syncResult))
	assert.Zero(t, syncResult.AppliedCount)
	require.Len(t, syncResult.Conflicts, 1)
	assert.Equal(t, model.SyncSchemaVersion, syncResult.SchemaVersion)
	require.NotNil(t, syncResult.Conflicts[0].ServerMessage)
	require.NotNil(t, syncResult.Conflicts[0].ClientMessage)
	assert.Equal(t, "Server version", syncResult.Conflicts[0].ServerMessage.Content)
	assert.Equal(t, "Client version", syncResult.Conflicts[0].ClientMessage.Content)

	storedConflict, err := app.MessageService.FindBySyncID(ctx, conflictSyncID)
	require.NoError(t, err)
	require.NotNil(t, storedConflict.SyncConflict)
	assert.Contains(t, *storedConflict.SyncConflict, "Client version")

	conflictPage := httptest.NewRecorder()
	assembled.ServeHTTP(conflictPage, web.WithUser(httptest.NewRequest(http.MethodGet, "/messages/resolve/"+conflictSyncID, nil), user))
	require.Equal(t, http.StatusOK, conflictPage.Code)
	assert.Contains(t, conflictPage.Body.String(), "Client version")
	assert.Contains(t, conflictPage.Body.String(), "Server version")

	resolveForm := url.Values{"strategy": {"mine"}}
	resolveRequest := httptest.NewRequest(http.MethodPost, "/messages/resolve/"+conflictSyncID, strings.NewReader(resolveForm.Encode()))
	resolveRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resolveResponse := httptest.NewRecorder()
	assembled.ServeHTTP(resolveResponse, web.WithUser(resolveRequest, user))
	require.Equal(t, http.StatusSeeOther, resolveResponse.Code)
	resolved, err := app.MessageService.FindBySyncID(ctx, conflictSyncID)
	require.NoError(t, err)
	assert.Equal(t, "Client version", resolved.Content)
	assert.Nil(t, resolved.SyncConflict)

	invalidSyncBody := strings.NewReader(`{"messages":[{"syncId":"","author":"searcher","content":"invalid","updatedAtEpochMs":1}]}`)
	invalidSyncRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sync", invalidSyncBody)
	invalidSyncRequest.Header.Set("Authorization", "Bearer "+syncToken)
	invalidSyncRequest.Header.Set("Content-Type", "application/json")
	invalidSync := httptest.NewRecorder()
	assembled.ServeHTTP(invalidSync, invalidSyncRequest)
	assert.Equal(t, http.StatusBadRequest, invalidSync.Code)

	createAPIKey := func(name string) (int, model.CreateApiKeyResponse) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", strings.NewReader(`{"name":"`+name+`"}`))
		request.Header.Set("Authorization", "Bearer "+syncToken)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)
		var result model.CreateApiKeyResponse
		if response.Code == http.StatusOK {
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		}
		return response.Code, result
	}

	blankAPIKeyStatus, _ := createAPIKey("   ")
	assert.Equal(t, http.StatusBadRequest, blankAPIKeyStatus)
	apiKeyStatus, apiKey := createAPIKey("Parity key")
	require.Equal(t, http.StatusOK, apiKeyStatus)
	assert.Len(t, apiKey.Key, 36)
	assert.True(t, strings.HasPrefix(apiKey.Key, "osk_"))
	assert.Len(t, apiKey.KeyPrefix, 8)
	assert.True(t, strings.HasPrefix(apiKey.Key, apiKey.KeyPrefix))

	var storedAPIKeyHash string
	require.NoError(t, pool.QueryRow(ctx, "SELECT key_hash FROM plt_api_keys WHERE user_id = $1 AND name = $2", user.ID, "Parity key").Scan(&storedAPIKeyHash))
	assert.Equal(t, security.NewTokenHasher(cfg.TokenPepper).Hash(apiKey.Key), storedAPIKeyHash)

	listAPIKeysRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/api-keys", nil)
	listAPIKeysRequest.Header.Set("Authorization", "Bearer "+syncToken)
	listAPIKeys := httptest.NewRecorder()
	assembled.ServeHTTP(listAPIKeys, listAPIKeysRequest)
	require.Equal(t, http.StatusOK, listAPIKeys.Code)
	var apiKeySummaries []model.ApiKeySummary
	require.NoError(t, json.NewDecoder(listAPIKeys.Body).Decode(&apiKeySummaries))
	require.Len(t, apiKeySummaries, 1)
	assert.Equal(t, "Parity key", apiKeySummaries[0].Name)

	apiKeySyncRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync?since=0", nil)
	apiKeySyncRequest.Header.Set("Authorization", "Bearer "+apiKey.Key)
	apiKeySync := httptest.NewRecorder()
	assembled.ServeHTTP(apiKeySync, apiKeySyncRequest)
	assert.Equal(t, http.StatusOK, apiKeySync.Code)

	_, err = pool.Exec(ctx, "UPDATE plt_users SET enabled = false WHERE id = $1", user.ID)
	require.NoError(t, err)
	disabledAPIKeySync := httptest.NewRecorder()
	disabledAPIKeySyncRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync?since=0", nil)
	disabledAPIKeySyncRequest.Header.Set("Authorization", "Bearer "+apiKey.Key)
	assembled.ServeHTTP(disabledAPIKeySync, disabledAPIKeySyncRequest)
	assert.Equal(t, http.StatusUnauthorized, disabledAPIKeySync.Code)
	_, err = pool.Exec(ctx, "UPDATE plt_users SET enabled = true WHERE id = $1", user.ID)
	require.NoError(t, err)

	missingAPIKeyDeleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/9223372036854775807", nil)
	missingAPIKeyDeleteRequest.Header.Set("Authorization", "Bearer "+syncToken)
	missingAPIKeyDelete := httptest.NewRecorder()
	assembled.ServeHTTP(missingAPIKeyDelete, missingAPIKeyDeleteRequest)
	assert.Equal(t, http.StatusOK, missingAPIKeyDelete.Code)

	deleteAPIKeyRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/"+strconv.FormatInt(apiKeySummaries[0].ID, 10), nil)
	deleteAPIKeyRequest.Header.Set("Authorization", "Bearer "+syncToken)
	deleteAPIKey := httptest.NewRecorder()
	assembled.ServeHTTP(deleteAPIKey, deleteAPIKeyRequest)
	assert.Equal(t, http.StatusOK, deleteAPIKey.Code)
	deletedAPIKeySync := httptest.NewRecorder()
	assembled.ServeHTTP(deletedAPIKeySync, apiKeySyncRequest.Clone(ctx))
	assert.Equal(t, http.StatusUnauthorized, deletedAPIKeySync.Code)

	apiJSON := func(method, path, token, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)
		return response
	}

	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodPost, "/api/v1/devices/register", "", `{"platform":"android","token":"device"}`).Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodPost, "/api/v1/devices/register", syncToken, `{"platform":"windows","token":"device"}`).Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodPost, "/api/v1/devices/register", syncToken, `{"platform":"android","token":"   "}`).Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodPost, "/api/v1/devices/register", syncToken, `{`).Code)

	deviceToken := "device-" + user.ID.String()
	deviceRegistration := apiJSON(http.MethodPost, "/api/v1/devices/register", syncToken,
		`{"platform":"android","token":"`+deviceToken+`","appBundle":"com.outerstellar.android"}`)
	require.Equal(t, http.StatusNoContent, deviceRegistration.Code)
	var storedDeviceUserID uuid.UUID
	var storedDevicePlatform, storedDeviceBundle string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT user_id, platform, app_bundle FROM plt_device_tokens WHERE token = $1", deviceToken,
	).Scan(&storedDeviceUserID, &storedDevicePlatform, &storedDeviceBundle))
	assert.Equal(t, user.ID, storedDeviceUserID)
	assert.Equal(t, "android", storedDevicePlatform)
	assert.Equal(t, "com.outerstellar.android", storedDeviceBundle)

	deviceRefresh := apiJSON(http.MethodPost, "/api/v1/devices/register", syncToken,
		`{"platform":"ios","token":"`+deviceToken+`","appBundle":"com.outerstellar.ios"}`)
	require.Equal(t, http.StatusNoContent, deviceRefresh.Code)
	var deviceCount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_device_tokens WHERE token = $1", deviceToken).Scan(&deviceCount))
	assert.Equal(t, 1, deviceCount)
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT platform, app_bundle FROM plt_device_tokens WHERE token = $1", deviceToken,
	).Scan(&storedDevicePlatform, &storedDeviceBundle))
	assert.Equal(t, "ios", storedDevicePlatform)
	assert.Equal(t, "com.outerstellar.ios", storedDeviceBundle)

	secondDeviceToken := "device-second-" + user.ID.String()
	require.Equal(t, http.StatusNoContent, apiJSON(http.MethodPost, "/api/v1/devices/register", syncToken,
		`{"platform":"android","token":"`+secondDeviceToken+`"}`).Code)
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_device_tokens WHERE user_id = $1", user.ID).Scan(&deviceCount))
	assert.Equal(t, 2, deviceCount)

	otherUser, err := app.SecurityService.Register(ctx, "parity-other", "OtherParity1!")
	require.NoError(t, err)
	otherToken, err := app.SecurityService.CreateSession(ctx, otherUser.ID)
	require.NoError(t, err)
	forbiddenDeviceDelete := apiJSON(http.MethodDelete, "/api/v1/devices/register", otherToken, `{"token":"`+deviceToken+`"}`)
	assert.Equal(t, http.StatusForbidden, forbiddenDeviceDelete.Code)
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_device_tokens WHERE token = $1", deviceToken).Scan(&deviceCount))
	assert.Equal(t, 1, deviceCount)

	require.Equal(t, http.StatusBadRequest, apiJSON(http.MethodDelete, "/api/v1/devices/register", syncToken, `{}`).Code)
	require.Equal(t, http.StatusNoContent, apiJSON(http.MethodDelete, "/api/v1/devices/register", syncToken, `{"token":"`+deviceToken+`"}`).Code)
	require.Equal(t, http.StatusNoContent, apiJSON(http.MethodDelete, "/api/v1/devices/register?token="+secondDeviceToken, syncToken, "").Code)
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_device_tokens WHERE user_id = $1", user.ID).Scan(&deviceCount))
	assert.Zero(t, deviceCount)

	pollCookieRequest := func(method, path, body, contentType string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: syncToken})
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)
		return response
	}

	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodPost, "/api/v1/polls", "", `{"question":"Choose?","options":["A","B"]}`).Code)
	assert.Equal(t, http.StatusBadRequest, pollCookieRequest(http.MethodPost, "/api/v1/polls",
		`{"question":"Choose?","options":["Only one"]}`, "application/json").Code)
	assert.Equal(t, http.StatusBadRequest, pollCookieRequest(http.MethodPost, "/api/v1/polls",
		`{"question":"Choose?","options":["A","B"],"deadline":"tomorrow"}`, "application/json").Code)

	pollCreate := pollCookieRequest(http.MethodPost, "/api/v1/polls",
		`{"question":"Choose a launch window?","options":["Morning","Evening"]}`, "application/json")
	require.Equal(t, http.StatusCreated, pollCreate.Code)
	var createdPoll model.PollWithResults
	require.NoError(t, json.NewDecoder(pollCreate.Body).Decode(&createdPoll))
	assert.Equal(t, "Choose a launch window?", createdPoll.Poll.Question)
	require.Len(t, createdPoll.Options, 2)
	assert.Empty(t, createdPoll.UserVotedOptionIDs)

	pollGet := pollCookieRequest(http.MethodGet, "/api/v1/polls/"+createdPoll.Poll.SyncID, "", "")
	require.Equal(t, http.StatusOK, pollGet.Code)
	var fetchedPoll model.PollWithResults
	require.NoError(t, json.NewDecoder(pollGet.Body).Decode(&fetchedPoll))
	assert.Equal(t, createdPoll.Poll.SyncID, fetchedPoll.Poll.SyncID)
	assert.Equal(t, http.StatusNotFound, pollCookieRequest(http.MethodGet, "/api/v1/polls/missing-poll", "", "").Code)

	pollCard := pollCookieRequest(http.MethodGet, "/components/polls/"+createdPoll.Poll.SyncID, "", "")
	require.Equal(t, http.StatusOK, pollCard.Code)
	assert.Contains(t, pollCard.Body.String(), "poll-card")
	assert.Contains(t, pollCard.Body.String(), "Choose a launch window?")

	firstOptionID := createdPoll.Options[0].ID
	secondOptionID := createdPoll.Options[1].ID
	pollVote := pollCookieRequest(http.MethodPost, "/api/v1/polls/"+createdPoll.Poll.SyncID+"/vote",
		`{"optionId":`+strconv.FormatInt(firstOptionID, 10)+`}`, "application/json")
	require.Equal(t, http.StatusOK, pollVote.Code)
	var votedPoll model.PollWithResults
	require.NoError(t, json.NewDecoder(pollVote.Body).Decode(&votedPoll))
	assert.Equal(t, int32(1), votedPoll.VoteCounts[firstOptionID])
	assert.Contains(t, votedPoll.UserVotedOptionIDs, firstOptionID)
	assert.Equal(t, http.StatusConflict, pollCookieRequest(http.MethodPost, "/api/v1/polls/"+createdPoll.Poll.SyncID+"/vote",
		`{"optionId":`+strconv.FormatInt(secondOptionID, 10)+`}`, "application/json").Code)

	pollRemoveVote := pollCookieRequest(http.MethodDelete,
		"/api/v1/polls/"+createdPoll.Poll.SyncID+"/vote?optionId="+strconv.FormatInt(firstOptionID, 10), "", "")
	require.Equal(t, http.StatusNoContent, pollRemoveVote.Code)
	pollComponentVote := pollCookieRequest(http.MethodPost, "/components/polls/"+createdPoll.Poll.SyncID+"/vote",
		"optionId="+strconv.FormatInt(secondOptionID, 10), "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusOK, pollComponentVote.Code)
	assert.Contains(t, pollComponentVote.Body.String(), "poll-card")

	assert.Equal(t, http.StatusForbidden, apiJSON(http.MethodPost,
		"/api/v1/polls/"+createdPoll.Poll.SyncID+"/close", otherToken, "").Code)
	assert.Equal(t, http.StatusOK, apiJSON(http.MethodPost,
		"/api/v1/polls/"+createdPoll.Poll.SyncID+"/close", syncToken, "").Code)
	closedPollVote := pollCookieRequest(http.MethodPost, "/components/polls/"+createdPoll.Poll.SyncID+"/vote",
		"optionId="+strconv.FormatInt(firstOptionID, 10), "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusConflict, closedPollVote.Code)
	assert.Contains(t, closedPollVote.Body.String(), "Poll is closed")

	assert.Equal(t, http.StatusNoContent, apiJSON(http.MethodDelete,
		"/api/v1/polls/"+createdPoll.Poll.SyncID, syncToken, "").Code)
	assert.Equal(t, http.StatusNotFound, pollCookieRequest(http.MethodGet,
		"/api/v1/polls/"+createdPoll.Poll.SyncID, "", "").Code)

	openPollCreate := pollCookieRequest(http.MethodPost, "/api/v1/polls",
		`{"question":"Visible open poll?","options":["Yes","No"]}`, "application/json")
	require.Equal(t, http.StatusCreated, openPollCreate.Code)
	openPollList := pollCookieRequest(http.MethodGet, "/api/v1/polls", "", "")
	require.Equal(t, http.StatusOK, openPollList.Code)
	assert.Contains(t, openPollList.Body.String(), "Visible open poll?")
	assert.NotContains(t, openPollList.Body.String(), "Choose a launch window?")

	adminUser, err := app.SecurityService.Register(ctx, "parity-admin", "AdminParity1!")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE plt_users SET role = 'ADMIN' WHERE id = $1", adminUser.ID)
	require.NoError(t, err)
	adminUser.Role = model.RoleAdmin
	adminToken, err := app.SecurityService.CreateSession(ctx, adminUser.ID)
	require.NoError(t, err)
	managedUser, err := app.SecurityService.Register(ctx, "parity-managed", "ManagedParity1!")
	require.NoError(t, err)
	managedToken, err := app.SecurityService.CreateSession(ctx, managedUser.ID)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodGet, "/api/v1/admin/users", "", "").Code)
	assert.Equal(t, http.StatusForbidden, apiJSON(http.MethodGet, "/api/v1/admin/users", managedToken, "").Code)
	adminUsers := apiJSON(http.MethodGet, "/api/v1/admin/users", adminToken, "")
	require.Equal(t, http.StatusOK, adminUsers.Code)
	var userSummaries []model.UserSummary
	require.NoError(t, json.NewDecoder(adminUsers.Body).Decode(&userSummaries))
	assert.Contains(t, userSummaries, managedUser.ToSummary())

	missingEnabled := apiJSON(http.MethodPut, "/api/v1/admin/users/"+managedUser.ID.String()+"/enabled", adminToken, `{}`)
	assert.Equal(t, http.StatusBadRequest, missingEnabled.Code)
	var managedEnabled bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT enabled FROM plt_users WHERE id = $1", managedUser.ID).Scan(&managedEnabled))
	assert.True(t, managedEnabled)

	var auditCountBefore int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_audit_log").Scan(&auditCountBefore))
	selfDisable := apiJSON(http.MethodPut, "/api/v1/admin/users/"+adminUser.ID.String()+"/enabled", adminToken, `{"enabled":false}`)
	assert.Equal(t, http.StatusBadRequest, selfDisable.Code)
	var auditCountAfter int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_audit_log").Scan(&auditCountAfter))
	assert.Equal(t, auditCountBefore, auditCountAfter)

	disableManaged := apiJSON(http.MethodPut, "/api/v1/admin/users/"+managedUser.ID.String()+"/enabled", adminToken, `{"enabled":false}`)
	require.Equal(t, http.StatusOK, disableManaged.Code)
	var auditAction, auditActor, auditTarget string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT action, actor_username, target_username
		FROM plt_audit_log ORDER BY created_at DESC, id DESC LIMIT 1
	`).Scan(&auditAction, &auditActor, &auditTarget))
	assert.Equal(t, "USER_DISABLED", auditAction)
	assert.Equal(t, adminUser.Username, auditActor)
	assert.Equal(t, managedUser.Username, auditTarget)
	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodPost, "/api/v1/auth/login", "",
		`{"username":"parity-managed","password":"ManagedParity1!"}`).Code)
	require.Equal(t, http.StatusOK, apiJSON(http.MethodPut,
		"/api/v1/admin/users/"+managedUser.ID.String()+"/enabled", adminToken, `{"enabled":true}`).Code)

	require.Equal(t, http.StatusOK, apiJSON(http.MethodPut,
		"/api/v1/admin/users/"+managedUser.ID.String()+"/role", adminToken, `{"role":"ADMIN"}`).Code)
	assert.Equal(t, http.StatusOK, apiJSON(http.MethodGet, "/api/v1/admin/users", managedToken, "").Code)
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT action, actor_username, target_username
		FROM plt_audit_log ORDER BY created_at DESC, id DESC LIMIT 1
	`).Scan(&auditAction, &auditActor, &auditTarget))
	assert.Equal(t, "USER_ROLE_CHANGED", auditAction)
	assert.Equal(t, managedUser.Username, auditTarget)
	require.Equal(t, http.StatusOK, apiJSON(http.MethodPut,
		"/api/v1/admin/users/"+managedUser.ID.String()+"/role", adminToken, `{"role":"USER"}`).Code)
	assert.Equal(t, http.StatusForbidden, apiJSON(http.MethodGet, "/api/v1/admin/users", managedToken, "").Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodPut,
		"/api/v1/admin/users/"+adminUser.ID.String()+"/role", adminToken, `{"role":"USER"}`).Code)
	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodPut, "/api/v1/auth/password", "",
		`{"currentPassword":"ManagedParity1!","newPassword":"ChangedParity2!"}`).Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodPut, "/api/v1/auth/password", managedToken,
		`{"currentPassword":"wrong-password","newPassword":"ChangedParity2!"}`).Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodPut, "/api/v1/auth/password", managedToken,
		`{"currentPassword":"ManagedParity1!","newPassword":"short"}`).Code)
	require.Equal(t, http.StatusOK, apiJSON(http.MethodPut, "/api/v1/auth/password", managedToken,
		`{"currentPassword":"ManagedParity1!","newPassword":"ChangedParity2!"}`).Code)
	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodPost, "/api/v1/auth/login", "",
		`{"username":"parity-managed","password":"ManagedParity1!"}`).Code)
	changedPasswordLogin := apiJSON(http.MethodPost, "/api/v1/auth/login", "",
		`{"username":"parity-managed","password":"ChangedParity2!"}`)
	assert.Equal(t, http.StatusOK, changedPasswordLogin.Code)

	adminPageRequest := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	adminPageRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: adminToken})
	adminPage := httptest.NewRecorder()
	assembled.ServeHTTP(adminPage, adminPageRequest)
	require.Equal(t, http.StatusOK, adminPage.Code)
	assert.Contains(t, adminPage.Body.String(), managedUser.Username)
	nonAdminPageRequest := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	nonAdminPageRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: managedToken})
	nonAdminPage := httptest.NewRecorder()
	assembled.ServeHTTP(nonAdminPage, nonAdminPageRequest)
	assert.Equal(t, http.StatusForbidden, nonAdminPage.Code)
	auditPageRequest := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	auditPageRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: adminToken})
	auditPage := httptest.NewRecorder()
	assembled.ServeHTTP(auditPage, auditPageRequest)
	require.Equal(t, http.StatusOK, auditPage.Code)
	assert.Contains(t, auditPage.Body.String(), "USER_ROLE_CHANGED")

	devDashboardRequest := httptest.NewRequest(http.MethodGet, "/admin/dev", nil)
	devDashboardRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: adminToken})
	devDashboard := httptest.NewRecorder()
	assembled.ServeHTTP(devDashboard, devDashboardRequest)
	require.Equal(t, http.StatusOK, devDashboard.Code)
	assert.Contains(t, devDashboard.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, devDashboard.Body.String(), "Outbox")
	assert.Contains(t, devDashboard.Body.String(), `action="/admin/dev/outbox/process"`)

	userDevDashboardRequest := httptest.NewRequest(http.MethodGet, "/admin/dev", nil)
	userDevDashboardRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: managedToken})
	userDevDashboard := httptest.NewRecorder()
	assembled.ServeHTTP(userDevDashboard, userDevDashboardRequest)
	assert.Equal(t, http.StatusForbidden, userDevDashboard.Code)

	anonymousDevDashboard := httptest.NewRecorder()
	assembled.ServeHTTP(anonymousDevDashboard, httptest.NewRequest(http.MethodGet, "/admin/dev", nil))
	assert.NotEqual(t, http.StatusOK, anonymousDevDashboard.Code)

	notificationUser, err := app.SecurityService.Register(ctx, "parity-notifications", "NotifyParity1!")
	require.NoError(t, err)
	notificationToken, err := app.SecurityService.CreateSession(ctx, notificationUser.ID)
	require.NoError(t, err)
	privateNotificationUser, err := app.SecurityService.Register(ctx, "parity-notifications-other", "NotifyOther1!")
	require.NoError(t, err)
	privateNotificationToken, err := app.SecurityService.CreateSession(ctx, privateNotificationUser.ID)
	require.NoError(t, err)
	initialNotifications := apiJSON(http.MethodGet, "/api/v1/notifications", notificationToken, "")
	require.Equal(t, http.StatusOK, initialNotifications.Code)
	assert.JSONEq(t, `[]`, initialNotifications.Body.String())
	require.NoError(t, app.NotificationService.Create(ctx, notificationUser.ID, "Hello", "First body", "info"))
	require.NoError(t, app.NotificationService.Create(ctx, notificationUser.ID, "Warning", "Second body", "warning"))
	require.NoError(t, app.NotificationService.Create(ctx, privateNotificationUser.ID, "Private", "Other body", "info"))

	listedNotifications := apiJSON(http.MethodGet, "/api/v1/notifications", notificationToken, "")
	require.Equal(t, http.StatusOK, listedNotifications.Code)
	var notificationSummaries []model.NotificationSummary
	require.NoError(t, json.NewDecoder(listedNotifications.Body).Decode(&notificationSummaries))
	require.Len(t, notificationSummaries, 2)
	assert.Equal(t, "Warning", notificationSummaries[0].Title)
	assert.False(t, notificationSummaries[0].Read)
	assert.NotContains(t, listedNotifications.Body.String(), "Private")

	foreignMarkRead := apiJSON(http.MethodPut,
		"/api/v1/notifications/"+notificationSummaries[0].ID+"/read", privateNotificationToken, "")
	assert.Equal(t, http.StatusNoContent, foreignMarkRead.Code)
	var foreignReadAt *time.Time
	require.NoError(t, pool.QueryRow(ctx, "SELECT read_at FROM plt_notifications WHERE id = $1", notificationSummaries[0].ID).Scan(&foreignReadAt))
	assert.Nil(t, foreignReadAt)
	assert.Equal(t, http.StatusNoContent, apiJSON(http.MethodPut,
		"/api/v1/notifications/"+uuid.NewString()+"/read", notificationToken, "").Code)

	markRead := apiJSON(http.MethodPut,
		"/api/v1/notifications/"+notificationSummaries[0].ID+"/read", notificationToken, "")
	require.Equal(t, http.StatusNoContent, markRead.Code)
	unreadCount := apiJSON(http.MethodGet, "/api/v1/notifications/unread-count", notificationToken, "")
	require.Equal(t, http.StatusOK, unreadCount.Code)
	assert.JSONEq(t, `{"count":1}`, unreadCount.Body.String())
	require.Equal(t, http.StatusNoContent,
		apiJSON(http.MethodPut, "/api/v1/notifications/read-all", notificationToken, "").Code)
	assert.JSONEq(t, `{"count":0}`,
		apiJSON(http.MethodGet, "/api/v1/notifications/unread-count", notificationToken, "").Body.String())
	require.Equal(t, http.StatusOK, apiJSON(http.MethodDelete,
		"/api/v1/notifications/"+notificationSummaries[0].ID, notificationToken, "").Code)

	bellRequest := httptest.NewRequest(http.MethodGet, "/components/notification-bell", nil)
	bellRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: notificationToken})
	bell := httptest.NewRecorder()
	assembled.ServeHTTP(bell, bellRequest)
	require.Equal(t, http.StatusOK, bell.Code)
	assert.Contains(t, bell.Body.String(), `href="/notifications"`)
	notificationPageRequest := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	notificationPageRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: notificationToken})
	notificationPage := httptest.NewRecorder()
	assembled.ServeHTTP(notificationPage, notificationPageRequest)
	require.Equal(t, http.StatusOK, notificationPage.Code)
	assert.Contains(t, notificationPage.Body.String(), "Hello")
	assert.Equal(t, http.StatusFound, apiJSON(http.MethodGet, "/notifications", "", "").Code)

	restoreMessage, err := app.MessageService.CreateServerMessage(ctx, "Restore Author", "Restore Content")
	require.NoError(t, err)
	require.NoError(t, app.MessageService.DeleteMessage(ctx, restoreMessage.SyncID))
	trashBeforeRequest := httptest.NewRequest(http.MethodGet, "/messages/trash", nil)
	trashBeforeRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: syncToken})
	trashBefore := httptest.NewRecorder()
	assembled.ServeHTTP(trashBefore, trashBeforeRequest)
	require.Equal(t, http.StatusOK, trashBefore.Code)
	assert.Contains(t, trashBefore.Body.String(), "Restore Author")
	homeBeforeRestoreRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	homeBeforeRestoreRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: syncToken})
	homeBeforeRestore := httptest.NewRecorder()
	assembled.ServeHTTP(homeBeforeRestore, homeBeforeRestoreRequest)
	assert.NotContains(t, homeBeforeRestore.Body.String(), "Restore Author")
	restoreRequest := httptest.NewRequest(http.MethodPost, "/messages/restore/"+restoreMessage.SyncID, nil)
	restoreRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: syncToken})
	restoreResponse := httptest.NewRecorder()
	assembled.ServeHTTP(restoreResponse, restoreRequest)
	require.Equal(t, http.StatusFound, restoreResponse.Code)
	assert.Equal(t, "/messages/trash", restoreResponse.Header().Get("Location"))
	trashAfter := httptest.NewRecorder()
	assembled.ServeHTTP(trashAfter, trashBeforeRequest.Clone(ctx))
	assert.NotContains(t, trashAfter.Body.String(), "Restore Author")
	homeAfterRestore := httptest.NewRecorder()
	assembled.ServeHTTP(homeAfterRestore, homeBeforeRestoreRequest.Clone(ctx))
	assert.Contains(t, homeAfterRestore.Body.String(), "Restore Author")
	unknownRestoreRequest := httptest.NewRequest(http.MethodPost, "/messages/restore/non-existent-sync-id", nil)
	unknownRestoreRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: syncToken})
	unknownRestore := httptest.NewRecorder()
	assembled.ServeHTTP(unknownRestore, unknownRestoreRequest)
	assert.Equal(t, http.StatusFound, unknownRestore.Code)
	assert.Equal(t, "/messages/trash", unknownRestore.Header().Get("Location"))

	profileUser, err := app.SecurityService.Register(ctx, "profile-parity", "ProfileParity1!")
	require.NoError(t, err)
	profileToken, err := app.SecurityService.CreateSession(ctx, profileUser.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodGet, "/api/v1/auth/profile", "", "").Code)
	initialProfileResponse := apiJSON(http.MethodGet, "/api/v1/auth/profile", profileToken, "")
	require.Equal(t, http.StatusOK, initialProfileResponse.Code)
	var initialProfile model.UserProfileResponse
	require.NoError(t, json.NewDecoder(initialProfileResponse.Body).Decode(&initialProfile))
	assert.Equal(t, profileUser.Username, initialProfile.Username)
	assert.Nil(t, initialProfile.AvatarURL)
	assert.True(t, initialProfile.EmailNotificationsEnabled)
	assert.True(t, initialProfile.PushNotificationsEnabled)

	apiProfileUpdate := apiJSON(http.MethodPut, "/api/v1/auth/profile", profileToken,
		`{"email":"profile-updated@example.com","username":"profile-updated","avatarUrl":"https://example.com/profile.png"}`)
	require.Equal(t, http.StatusOK, apiProfileUpdate.Code)
	updatedProfileResponse := apiJSON(http.MethodGet, "/api/v1/auth/profile", profileToken, "")
	require.Equal(t, http.StatusOK, updatedProfileResponse.Code)
	var updatedProfile model.UserProfileResponse
	require.NoError(t, json.NewDecoder(updatedProfileResponse.Body).Decode(&updatedProfile))
	assert.Equal(t, "profile-updated", updatedProfile.Username)
	assert.Equal(t, "profile-updated@example.com", updatedProfile.Email)
	require.NotNil(t, updatedProfile.AvatarURL)
	assert.Equal(t, "https://example.com/profile.png", *updatedProfile.AvatarURL)
	duplicateProfileUpdate := apiJSON(http.MethodPut, "/api/v1/auth/profile", profileToken,
		`{"email":"profile-updated@example.com","username":"parity-other"}`)
	assert.Equal(t, http.StatusConflict, duplicateProfileUpdate.Code)

	require.Equal(t, http.StatusOK, apiJSON(http.MethodPut, "/api/v1/auth/notification-preferences", profileToken,
		`{"emailEnabled":false,"pushEnabled":false}`).Code)
	require.Equal(t, http.StatusOK, apiJSON(http.MethodPut, "/api/v1/auth/notification-preferences", profileToken,
		`{"emailEnabled":true,"pushEnabled":false}`).Code)
	preferencesProfileResponse := apiJSON(http.MethodGet, "/api/v1/auth/profile", profileToken, "")
	var preferencesProfile model.UserProfileResponse
	require.NoError(t, json.NewDecoder(preferencesProfileResponse.Body).Decode(&preferencesProfile))
	assert.True(t, preferencesProfile.EmailNotificationsEnabled)
	assert.False(t, preferencesProfile.PushNotificationsEnabled)

	deleteAPIUser, err := app.SecurityService.Register(ctx, "delete-api-parity", "DeleteAPIParity1!")
	require.NoError(t, err)
	deleteAPIToken, err := app.SecurityService.CreateSession(ctx, deleteAPIUser.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodDelete, "/api/v1/auth/account", deleteAPIToken, "").Code)
	assert.Equal(t, http.StatusBadRequest, apiJSON(http.MethodDelete, "/api/v1/auth/account", deleteAPIToken,
		`{"currentPassword":"wrong-password"}`).Code)
	require.Equal(t, http.StatusOK, apiJSON(http.MethodDelete, "/api/v1/auth/account", deleteAPIToken,
		`{"currentPassword":"DeleteAPIParity1!"}`).Code)
	var deletedAPICount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_users WHERE id = $1", deleteAPIUser.ID).Scan(&deletedAPICount))
	assert.Zero(t, deletedAPICount)

	soleAdmin, err := app.SecurityService.Register(ctx, "sole-delete-admin", "SoleAdminParity1!")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE plt_users SET role = 'ADMIN' WHERE id = $1", soleAdmin.ID)
	require.NoError(t, err)
	soleAdminToken, err := app.SecurityService.CreateSession(ctx, soleAdmin.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE plt_users SET role = 'USER' WHERE id = $1", adminUser.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, apiJSON(http.MethodDelete, "/api/v1/auth/account", soleAdminToken,
		`{"currentPassword":"SoleAdminParity1!"}`).Code)
	_, err = pool.Exec(ctx, "UPDATE plt_users SET role = 'ADMIN' WHERE id = $1", adminUser.ID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, apiJSON(http.MethodDelete, "/api/v1/auth/account", soleAdminToken,
		`{"currentPassword":"SoleAdminParity1!"}`).Code)

	passwordUser, err := app.SecurityService.Register(ctx, "password-component-parity", "OldPassword1!")
	require.NoError(t, err)
	passwordToken, err := app.SecurityService.CreateSession(ctx, passwordUser.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, apiJSON(http.MethodGet, "/auth/change-password", "", "").Code)
	changePasswordPageRequest := httptest.NewRequest(http.MethodGet, "/auth/change-password", nil)
	changePasswordPageRequest.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: passwordToken})
	changePasswordPage := httptest.NewRecorder()
	assembled.ServeHTTP(changePasswordPage, changePasswordPageRequest)
	require.Equal(t, http.StatusOK, changePasswordPage.Code)
	assert.Contains(t, strings.ToLower(changePasswordPage.Body.String()), "html")
	assert.Equal(t, http.StatusFound, apiJSON(http.MethodPost, "/auth/components/change-password", "",
		"currentPassword=old&newPassword=new&confirmPassword=new").Code)
	changePasswordComponent := func(currentPassword, newPassword, confirmPassword string) *httptest.ResponseRecorder {
		form := url.Values{
			"currentPassword": {currentPassword},
			"newPassword":     {newPassword},
			"confirmPassword": {confirmPassword},
		}
		request := httptest.NewRequest(http.MethodPost, "/auth/components/change-password", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: passwordToken})
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)
		return response
	}
	mismatchedPassword := changePasswordComponent("OldPassword1!", "NewPassword2!", "DifferentPassword2!")
	require.Equal(t, http.StatusOK, mismatchedPassword.Code)
	assert.Contains(t, mismatchedPassword.Body.String(), "auth-result-error")
	wrongPassword := changePasswordComponent("WrongPassword1!", "NewPassword2!", "NewPassword2!")
	require.Equal(t, http.StatusOK, wrongPassword.Code)
	assert.Contains(t, wrongPassword.Body.String(), "auth-result-error")
	weakPassword := changePasswordComponent("OldPassword1!", "weak", "weak")
	require.Equal(t, http.StatusOK, weakPassword.Code)
	assert.Contains(t, weakPassword.Body.String(), "auth-result-error")
	successfulPassword := changePasswordComponent("OldPassword1!", "NewPassword2!", "NewPassword2!")
	require.Equal(t, http.StatusOK, successfulPassword.Code)
	assert.Contains(t, successfulPassword.Body.String(), "auth-result-success")
	assert.Equal(t, http.StatusUnauthorized, apiJSON(http.MethodPost, "/api/v1/auth/login", "",
		`{"username":"password-component-parity","password":"OldPassword1!"}`).Code)
	assert.Equal(t, http.StatusOK, apiJSON(http.MethodPost, "/api/v1/auth/login", "",
		`{"username":"password-component-parity","password":"NewPassword2!"}`).Code)

	pushContactSync := func(contact model.SyncContact) (int, model.SyncPushContactResponse) {
		payload, marshalErr := json.Marshal(model.SyncPushContactRequest{Contacts: []model.SyncContact{contact}})
		require.NoError(t, marshalErr)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/sync/contacts", strings.NewReader(string(payload)))
		request.Header.Set("Authorization", "Bearer "+syncToken)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)

		var result model.SyncPushContactResponse
		if response.Code == http.StatusOK {
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		}
		return response.Code, result
	}
	pullContacts := func(since int64) model.SyncPullContactResponse {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/contacts?since="+strconv.FormatInt(since, 10), nil)
		request.Header.Set("Authorization", "Bearer "+syncToken)
		response := httptest.NewRecorder()
		assembled.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		var result model.SyncPullContactResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		return result
	}
	findPulledContact := func(result model.SyncPullContactResponse, syncID string) *model.SyncContact {
		for i := range result.Contacts {
			if result.Contacts[i].SyncID == syncID {
				return &result.Contacts[i]
			}
		}
		return nil
	}

	contactSyncID := "contact-sync-" + user.ID.String()[:16]
	contactV1 := model.SyncContact{
		SyncID:           contactSyncID,
		Name:             "Contact version one",
		Emails:           []string{"home@example.com", "work@example.com"},
		Phones:           []string{"+1-555-0100", "+1-555-0200"},
		SocialMedia:      []string{"@contact", "example.com/contact"},
		Company:          "First Company",
		CompanyAddress:   "1 First Street",
		Department:       "Research",
		UpdatedAtEpochMs: 2000,
	}
	status, contactPush := pushContactSync(contactV1)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, 1, contactPush.AppliedCount)
	assert.Empty(t, contactPush.Conflicts)

	pulledContact := findPulledContact(pullContacts(0), contactSyncID)
	require.NotNil(t, pulledContact)
	assert.Equal(t, contactV1.Name, pulledContact.Name)
	assert.ElementsMatch(t, contactV1.Emails, pulledContact.Emails)
	assert.ElementsMatch(t, contactV1.Phones, pulledContact.Phones)
	assert.ElementsMatch(t, contactV1.SocialMedia, pulledContact.SocialMedia)
	assert.Equal(t, contactV1.Company, pulledContact.Company)
	assert.Equal(t, contactV1.CompanyAddress, pulledContact.CompanyAddress)
	assert.Equal(t, contactV1.Department, pulledContact.Department)

	contactV2 := contactV1
	contactV2.Name = "Contact version two"
	contactV2.Emails = []string{"new@example.com", "backup@example.com"}
	contactV2.Phones = []string{"+44-20-1234-5678"}
	contactV2.SocialMedia = []string{"@new-contact"}
	contactV2.Company = "Second Company"
	contactV2.CompanyAddress = "2 Second Street"
	contactV2.Department = "Engineering"
	contactV2.UpdatedAtEpochMs = 3000
	status, contactPush = pushContactSync(contactV2)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, 1, contactPush.AppliedCount)

	pulledContact = findPulledContact(pullContacts(0), contactSyncID)
	require.NotNil(t, pulledContact)
	assert.Equal(t, contactV2.Name, pulledContact.Name)
	assert.ElementsMatch(t, contactV2.Emails, pulledContact.Emails)
	assert.ElementsMatch(t, contactV2.Phones, pulledContact.Phones)
	assert.ElementsMatch(t, contactV2.SocialMedia, pulledContact.SocialMedia)
	assert.NotContains(t, pulledContact.Emails, "home@example.com")
	assert.Equal(t, contactV2.Company, pulledContact.Company)
	assert.Equal(t, contactV2.CompanyAddress, pulledContact.CompanyAddress)
	assert.Equal(t, contactV2.Department, pulledContact.Department)

	staleContact := contactV1
	staleContact.Name = "Stale contact"
	staleContact.UpdatedAtEpochMs = 2500
	status, contactPush = pushContactSync(staleContact)
	require.Equal(t, http.StatusOK, status)
	assert.Zero(t, contactPush.AppliedCount)
	require.Len(t, contactPush.Conflicts, 1)
	require.NotNil(t, contactPush.Conflicts[0].ServerContact)
	assert.Equal(t, contactV2.Name, contactPush.Conflicts[0].ServerContact.Name)
	assert.ElementsMatch(t, contactV2.Emails, contactPush.Conflicts[0].ServerContact.Emails)

	deletedContact := contactV2
	deletedContact.Deleted = true
	deletedContact.UpdatedAtEpochMs = 4000
	status, contactPush = pushContactSync(deletedContact)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, 1, contactPush.AppliedCount)
	pulledContact = findPulledContact(pullContacts(3000), contactSyncID)
	require.NotNil(t, pulledContact)
	assert.True(t, pulledContact.Deleted)

	emptyContactPushBody := strings.NewReader(`{"contacts":[]}`)
	emptyContactPushRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sync/contacts", emptyContactPushBody)
	emptyContactPushRequest.Header.Set("Authorization", "Bearer "+syncToken)
	emptyContactPushRequest.Header.Set("Content-Type", "application/json")
	emptyContactPush := httptest.NewRecorder()
	assembled.ServeHTTP(emptyContactPush, emptyContactPushRequest)
	require.Equal(t, http.StatusOK, emptyContactPush.Code)
	var emptyContactResult model.SyncPushContactResponse
	require.NoError(t, json.NewDecoder(emptyContactPush.Body).Decode(&emptyContactResult))
	assert.Zero(t, emptyContactResult.AppliedCount)
	assert.Empty(t, emptyContactResult.Conflicts)
	assert.Empty(t, pullContacts(int64(^uint64(0)>>1)-1).Contacts)

	concurrentSyncID := "concurrent-sync-" + user.ID.String()[:16]
	statuses := make(chan int, 2)
	var pushes sync.WaitGroup
	for i := 0; i < 2; i++ {
		pushes.Add(1)
		go func(version int) {
			defer pushes.Done()
			body := strings.NewReader(`{"messages":[{"syncId":"` + concurrentSyncID + `","author":"searcher","content":"Version ` + strconv.Itoa(version) + `","updatedAtEpochMs":` + strconv.Itoa(5000+version) + `}]}`)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sync", body)
			request.Header.Set("Authorization", "Bearer "+syncToken)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			assembled.ServeHTTP(response, request)
			statuses <- response.Code
		}(i)
	}
	pushes.Wait()
	close(statuses)
	for status := range statuses {
		assert.Equal(t, http.StatusOK, status)
	}
	var concurrentCount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_messages WHERE sync_id = $1", concurrentSyncID).Scan(&concurrentCount))
	assert.Equal(t, 1, concurrentCount)

	_, err = pool.Exec(ctx, `
		INSERT INTO plt_messages (sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version)
		SELECT 'batch-message-' || n, 'batch', 'message', 10000 + n, false, false, 1
		FROM generate_series(1, 501) AS n`)
	require.NoError(t, err)
	messagePullRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync?since=9999", nil)
	messagePullRequest.Header.Set("Authorization", "Bearer "+syncToken)
	messagePull := httptest.NewRecorder()
	assembled.ServeHTTP(messagePull, messagePullRequest)
	require.Equal(t, http.StatusOK, messagePull.Code)
	var messagePullResult model.SyncPullResponse
	require.NoError(t, json.NewDecoder(messagePull.Body).Decode(&messagePullResult))
	assert.Len(t, messagePullResult.Messages, 500)
	assert.True(t, messagePullResult.HasMore)
	assert.Equal(t, model.SyncSchemaVersion, messagePullResult.SchemaVersion)

	_, err = pool.Exec(ctx, `
		INSERT INTO plt_contacts (sync_id, name, updated_at_epoch_ms, dirty, deleted, version)
		SELECT 'batch-contact-' || n, 'Batch contact', 10000 + n, false, false, 1
		FROM generate_series(1, 501) AS n`)
	require.NoError(t, err)
	contactPullRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync/contacts?since=9999", nil)
	contactPullRequest.Header.Set("Authorization", "Bearer "+syncToken)
	contactPull := httptest.NewRecorder()
	assembled.ServeHTTP(contactPull, contactPullRequest)
	require.Equal(t, http.StatusOK, contactPull.Code)
	var contactPullResult model.SyncPullContactResponse
	require.NoError(t, json.NewDecoder(contactPull.Body).Decode(&contactPullResult))
	assert.Len(t, contactPullResult.Contacts, 500)
	assert.True(t, contactPullResult.HasMore)

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
