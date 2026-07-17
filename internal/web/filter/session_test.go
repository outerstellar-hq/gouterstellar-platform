package filter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type sessionLookupStub func(context.Context, string) model.SessionLookup

func (s sessionLookupStub) LookupSession(ctx context.Context, token string) model.SessionLookup {
	return s(ctx, token)
}

func TestExpiredSessionCookieRedirectsBrowserAndClearsCookie(t *testing.T) {
	handler := Session(sessionLookupStub(func(context.Context, string) model.SessionLookup {
		return model.SessionExpired{}
	}), false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("expired session reached downstream handler")
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: "oss_expired"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	assert.Equal(t, "/auth?expired=true", recorder.Header().Get("Location"))
	assert.Equal(t, "true", recorder.Header().Get(SessionExpiredHeader))
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, web.SessionCookieName, cookies[0].Name)
	assert.LessOrEqual(t, cookies[0].MaxAge, 0)
}

func TestExpiredSessionCookieReturnsAPIUnauthorized(t *testing.T) {
	handler := Session(sessionLookupStub(func(context.Context, string) model.SessionLookup {
		return model.SessionExpired{}
	}), false)(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	request.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: "oss_expired"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, "true", recorder.Header().Get(SessionExpiredHeader))
	assert.Contains(t, recorder.Body.String(), "Session expired")
}

func TestExpiredWebSocketSessionReachesProtocolHandler(t *testing.T) {
	reached := false
	handler := Session(sessionLookupStub(func(context.Context, string) model.SessionLookup {
		return model.SessionExpired{}
	}), false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	request := httptest.NewRequest(http.MethodGet, "/ws/sync", nil)
	request.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: "oss_expired"})
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.True(t, reached)
	assert.Equal(t, http.StatusSwitchingProtocols, recorder.Code)
}

func TestActiveSessionRefreshesCookieAndPopulatesUser(t *testing.T) {
	user := &model.User{ID: uuid.New(), Username: "active"}
	handler := Session(sessionLookupStub(func(_ context.Context, token string) model.SessionLookup {
		assert.Equal(t, "oss_active", token)
		return model.SessionActive{User: user}
	}), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, user, web.UserFromRequest(r))
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: "oss_active"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	setCookie := recorder.Result().Cookies()
	require.Len(t, setCookie, 1)
	assert.Equal(t, "oss_active", setCookie[0].Value)
}

func TestUnauthenticatedRedirectPreservesSameOriginDestination(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/admin/users?view=disabled", nil)
	recorder := httptest.NewRecorder()

	RequireAuthenticated(http.NotFoundHandler()).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	location, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/auth", location.Path)
	assert.Equal(t, "/admin/users?view=disabled", location.Query().Get("returnTo"))
}
