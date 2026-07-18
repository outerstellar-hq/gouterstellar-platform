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

func TestSessionsRevokeSubjectDeletesOnlyMatchingSessions(t *testing.T) {
	store := memstore.New()
	sessions, err := NewSessions(store, PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}

	issue := func(subject string) *http.Cookie {
		handler := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := sessions.SignIn(r.Context(), subject); err != nil {
				t.Fatal(err)
			}
		}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/login", nil))
		return response.Result().Cookies()[0]
	}
	revoked := issue("user-1")
	kept := issue("user-2")

	if err := sessions.RevokeSubject(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	assertStatus := func(cookie *http.Cookie, want int) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		sessions.Middleware(RequireAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))).ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("status=%d want=%d", response.Code, want)
		}
	}
	assertStatus(revoked, http.StatusUnauthorized)
	assertStatus(kept, http.StatusNoContent)
}

func TestSessionsRevokeSubjectWithStoreUsesConsumerTransaction(t *testing.T) {
	store := memstore.New()
	sessions, err := NewSessions(store, PrincipalResolverFunc(func(_ context.Context, subject string) (Principal, error) {
		return Principal{Subject: subject}, nil
	}), SessionConfig{CookieName: "test_session", AllowInsecureCookies: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"user-1", "user-1", "user-2"} {
		handler := sessions.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := sessions.SignIn(r.Context(), subject); err != nil {
				t.Fatal(err)
			}
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/login", nil))
	}
	all, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	transaction := &testRevocationStore{sessions: all}
	if err := sessions.RevokeSubjectWithStore(context.Background(), "user-1", transaction); err != nil {
		t.Fatal(err)
	}
	if len(transaction.deleted) != 2 || len(transaction.sessions) != 1 {
		t.Fatalf("deleted=%d remaining=%d", len(transaction.deleted), len(transaction.sessions))
	}
}

func TestPrincipalClaimsAreCopiedIntoContext(t *testing.T) {
	claims := map[string]string{"email": "user@example.test"}
	ctx := withPrincipal(context.Background(), Principal{Subject: "user-1", Claims: claims})
	claims["email"] = "mutated@example.test"
	principal, ok := PrincipalFromContext(ctx)
	if !ok || principal.Claims["email"] != "user@example.test" {
		t.Fatalf("principal = %#v", principal)
	}
}

type testRevocationStore struct {
	sessions map[string][]byte
	deleted  []string
}

func (s *testRevocationStore) All(context.Context) (map[string][]byte, error) {
	return s.sessions, nil
}

func (s *testRevocationStore) Delete(_ context.Context, token string) error {
	delete(s.sessions, token)
	s.deleted = append(s.deleted, token)
	return nil
}
