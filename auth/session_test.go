package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2/memstore"
)

func TestSessionsSignInResolveAndAuthorize(t *testing.T) {
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject, Roles: []string{"admin"}}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), "user-1"); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	loginResponse := httptest.NewRecorder()
	login.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/login", nil))
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "test_session" || !cookies[0].HttpOnly || cookies[0].Secure {
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

func TestSessionsFailClosedAndClearDisabledPrincipal(t *testing.T) {
	sessions, err := NewSessions(memstore.New(), PrincipalResolverFunc(func(context.Context, string) (Principal, error) {
		return Principal{}, ErrUnauthenticated
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	login := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignIn(r.Context(), "disabled-user"); err != nil {
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
		if err := sessions.SignIn(r.Context(), "user-1"); err != nil {
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
