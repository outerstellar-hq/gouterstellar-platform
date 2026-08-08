package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2/memstore"
)

const testSecurityVersion SecurityVersion = "version-1"

func TestSessionsSignInResolveAndAuthorize(t *testing.T) {
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return NewPrincipal(subject, testSecurityVersion, []string{"admin"}, nil), nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), testSessionIdentity("user-1")); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	loginResponse := httptest.NewRecorder()
	login.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/login", nil))
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "test_session" || !cookies[0].HttpOnly || cookies[0].Secure ||
		cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}

	protected := sessions.Middleware(RequireRole("admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.Subject != "user-1" {
			t.Fatalf("principal = %#v, %v", principal, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookies[0])
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestSessionsValidateAndApplySameSitePolicy(t *testing.T) {
	resolver := PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject, SecurityVersion: testSecurityVersion}, nil
	})
	if _, err := NewSessions(memstore.New(), resolver, SessionConfig{
		CookieName: "test_session", SameSite: http.SameSite(99),
	}); err == nil {
		t.Fatal("expected invalid SameSite error")
	}
	if _, err := NewSessions(memstore.New(), resolver, SessionConfig{
		CookieName: "test_session", Lifetime: -time.Second,
	}); err == nil {
		t.Fatal("expected negative lifetime error")
	}
	if _, err := NewSessions(memstore.New(), resolver, SessionConfig{
		CookieName: "test_session", SameSite: http.SameSiteNoneMode, AllowInsecureCookies: true,
	}); err == nil {
		t.Fatal("expected insecure SameSite=None error")
	}

	sessions, err := NewSessions(memstore.New(), resolver, SessionConfig{
		CookieName: "test_session", SameSite: http.SameSiteStrictMode, AllowInsecureCookies: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions.manager.Put(r.Context(), "value", "present")
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}
}

func TestSessionsSignInRotatesExistingToken(t *testing.T) {
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject, SecurityVersion: testSecurityVersion}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	start := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions.manager.Put(r.Context(), "cart", "item")
		w.WriteHeader(http.StatusNoContent)
	}))
	startResponse := httptest.NewRecorder()
	start.ServeHTTP(startResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	original := startResponse.Result().Cookies()[0]

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), testSessionIdentity("user-1")); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	request.AddCookie(original)
	loginResponse := httptest.NewRecorder()
	login.ServeHTTP(loginResponse, request)
	rotated := loginResponse.Result().Cookies()[0]
	if original.Value == rotated.Value {
		t.Fatal("sign-in did not rotate the session token")
	}
}

func TestSessionsSignOutDestroysAuthenticatedSession(t *testing.T) {
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject, SecurityVersion: testSecurityVersion}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), testSessionIdentity("user-1")); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	loginResponse := httptest.NewRecorder()
	login.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/login", nil))
	sessionCookie := loginResponse.Result().Cookies()[0]

	logout := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignOut(r.Context()); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	logoutRequest := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutRequest.AddCookie(sessionCookie)
	logoutResponse := httptest.NewRecorder()
	logout.ServeHTTP(logoutResponse, logoutRequest)
	cleared := logoutResponse.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("session cookie was not expired: %#v", cleared)
	}

	protected := sessions.Middleware(RequireAuthenticated(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("destroyed session remained authenticated")
	})))
	staleRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	staleRequest.AddCookie(sessionCookie)
	staleResponse := httptest.NewRecorder()
	protected.ServeHTTP(staleResponse, staleRequest)
	if staleResponse.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", staleResponse.Code, http.StatusUnauthorized)
	}
}

func TestSessionsFailClosedAndClearDisabledPrincipal(t *testing.T) {
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(context.Context, string) (Principal, error) {
		return Principal{}, ErrUnauthenticated
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), testSessionIdentity("disabled-user")); err != nil {
			t.Fatal(err)
		}
	}))
	loginResponse := httptest.NewRecorder()
	login.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/login", nil))
	cookie := loginResponse.Result().Cookies()[0]

	protected := sessions.Middleware(RequireAuthenticated(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	cleared := response.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("session cookie was not cleared: %#v", cleared)
	}
}

func TestSessionsInfrastructureFailureDoesNotReachHandler(t *testing.T) {
	handled := false
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(context.Context, string) (Principal, error) {
		return Principal{}, errors.New("database unavailable")
	}), SessionConfig{
		CookieName:           "test_session",
		AllowInsecureCookies: true,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			if !strings.Contains(err.Error(), "database unavailable") {
				t.Fatalf("error = %v", err)
			}
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), testSessionIdentity("user-1")); err != nil {
			t.Fatal(err)
		}
	}))
	loginResponse := httptest.NewRecorder()
	login.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/login", nil))
	cookie := loginResponse.Result().Cookies()[0]

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	sessions.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { handled = true })).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || handled {
		t.Fatalf("status=%d handled=%v", response.Code, handled)
	}
}

func TestSessionsSecurityVersionRejectsExistingAndDelayedStaleSessions(t *testing.T) {
	currentVersion := SecurityVersion("version-1")
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject, SecurityVersion: currentVersion}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	issue := func(identity SessionIdentity) *http.Cookie {
		handler := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := sessions.SignIn(r.Context(), identity); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/login", nil))
		return response.Result().Cookies()[0]
	}
	assertStatus := func(cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		sessions.Middleware(RequireAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))).ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("status=%d want=%d", response.Code, want)
		}
		return response
	}

	original := issue(SessionIdentity{Subject: "user-1", SecurityVersion: currentVersion})
	assertStatus(original, http.StatusNoContent)

	staleVersion := currentVersion
	currentVersion = "version-2"
	staleResponse := assertStatus(original, http.StatusUnauthorized)
	if cookies := staleResponse.Result().Cookies(); len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("stale session cookie was not cleared: %#v", cookies)
	}

	delayed := issue(SessionIdentity{Subject: "user-1", SecurityVersion: staleVersion})
	assertStatus(delayed, http.StatusUnauthorized)
	current := issue(SessionIdentity{Subject: "user-1", SecurityVersion: currentVersion})
	assertStatus(current, http.StatusNoContent)
}

func TestSessionsMissingSecurityVersionFailsClosed(t *testing.T) {
	currentVersion := testSecurityVersion
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject, SecurityVersion: currentVersion}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.SignIn(context.Background(), SessionIdentity{Subject: "user-1"}); err == nil {
		t.Fatal("expected missing security version error")
	}

	issue := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.manager.RenewToken(r.Context()); err != nil {
			t.Fatal(err)
		}
		sessions.manager.Put(r.Context(), sessionSubjectKey, "user-1")
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	issue.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/legacy-login", nil))
	cookie := response.Result().Cookies()[0]

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	protectedResponse := httptest.NewRecorder()
	sessions.Middleware(RequireAuthenticated(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))).ServeHTTP(
		protectedResponse, request,
	)
	if protectedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unversioned session status = %d", protectedResponse.Code)
	}

	currentVersion = ""
	versioned := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), testSessionIdentity("user-1")); err != nil {
			t.Fatal(err)
		}
	}))
	versionedResponse := httptest.NewRecorder()
	versioned.ServeHTTP(versionedResponse, httptest.NewRequest(http.MethodPost, "/login", nil))
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(versionedResponse.Result().Cookies()[0])
	protectedResponse = httptest.NewRecorder()
	sessions.Middleware(RequireAuthenticated(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))).ServeHTTP(
		protectedResponse, request,
	)
	if protectedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missing current version status = %d", protectedResponse.Code)
	}
}

func TestPrincipalMutableFieldsAreIsolatedFromContext(t *testing.T) {
	roles := []string{"admin"}
	claims := map[string]string{"email": "user@example.test"}
	ctx := withPrincipal(context.Background(), NewPrincipal("user-1", "", roles, claims))
	roles[0] = "mutated-before-read"
	claims["email"] = "mutated@example.test"
	principal, ok := PrincipalFromContext(ctx)
	readRoles := principal.Roles()
	readClaims := principal.Claims()
	if !ok || readRoles[0] != "admin" || readClaims["email"] != "user@example.test" {
		t.Fatalf("principal = %#v", principal)
	}
	readRoles[0] = "mutated-after-read"
	readClaims["email"] = "mutated-after-read@example.test"
	second, ok := PrincipalFromContext(ctx)
	if !ok || second.Roles()[0] != "admin" || second.Claims()["email"] != "user@example.test" {
		t.Fatalf("context principal was mutated through returned value: %#v", second)
	}
}

func testSessionIdentity(subject string) SessionIdentity {
	return SessionIdentity{Subject: subject, SecurityVersion: testSecurityVersion}
}
