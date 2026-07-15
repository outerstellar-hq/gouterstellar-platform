package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

// stubServerExtension is a minimal extension that registers the small set of
// routes the real-server tests drive: a public /health, a public /auth, a
// protected / (home), and a static-asset handler under /static. It owns its
// route prefixes so NewHandler's ownership validation passes.
type stubServerExtension struct{}

func (stubServerExtension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "server-test-ext",
		Label: "Server Test",
		Mode:  extplatform.FullPlatform,
		Ownership: extplatform.RouteOwnership{
			UI:     []string{"/", "/auth"},
			API:    []string{"/api"},
			Admin:  []string{"/admin"},
			Assets: []string{"/static"},
		},
	}
}

func (stubServerExtension) Contribute(ctx *extplatform.ContributionContext) error {
	// Public: health check (unauthenticated, like the production /health).
	ctx.Routes.Public(http.MethodGet, "/health", "health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"healthy"}`)
	}))

	// Public: auth page.
	ctx.Routes.Public(http.MethodGet, "/auth", "login page", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html>login</html>")
	}))

	// Protected: home dashboard. In production this is guarded by the
	// GroupProtectedUI middleware chain (see cmd/server/main.go), which is
	// wired here via NewHandler's GroupMiddleware so the test exercises the
	// real redirect path.
	ctx.Routes.Protected(http.MethodGet, "/", "home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := web.UserFromRequest(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if user != nil {
			_, _ = io.WriteString(w, "<html>welcome "+user.Username+"</html>")
		} else {
			_, _ = io.WriteString(w, "<html>home</html>")
		}
	}))

	// Assets: serve a tiny in-memory file so we can assert 200 on a static path.
	ctx.Routes.Assets("/static/*", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "body { color: black; }")
	})))

	return nil
}

// newServerTestHandler assembles a handler with the same protected-route
// guard the production wire root applies to the admin group
// (filter.RequirePermission), but attached to GroupProtectedUI so the
// unauthenticated-redirect behaviour is exercised end-to-end through a real
// http.Server.
func newServerTestHandler(t *testing.T) http.Handler {
	t.Helper()
	permissionResolver := security.NewRoleBasedPermissionResolver()
	h, err := extplatform.NewHandler(extplatform.Options{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{stubServerExtension{}},
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			// This mirrors how cmd/server guards admin routes; applied to the
			// protected UI group it produces the /auth redirect that the
			// browser sees when hitting / without a session.
			extplatform.GroupProtectedUI: {filter.RequirePermission(permissionResolver, "*", "*")},
		},
	})
	require.NoError(t, err)
	return h
}

// TestHealthEndpoint returns 200 for the public health route through a real
// httptest.NewServer. This proves the server wires up, the route resolves,
// and the response flows back to a real client.
func TestHealthEndpoint(t *testing.T) {
	handler := newServerTestHandler(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "healthy")
}

// TestUnauthenticatedRedirectToAuth verifies the core Tier 4 behaviour: an
// unauthenticated GET / is redirected to /auth. The redirect is produced by
// filter.RequirePermission (the same middleware cmd/server applies to the
// admin group) attached here to GroupProtectedUI.
func TestUnauthenticatedRedirectToAuth(t *testing.T) {
	handler := newServerTestHandler(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Use a client that does NOT follow redirects so we can inspect the 303.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	// filter.RequirePermission issues a 303 See Other to /auth for browser routes.
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	loc := resp.Header.Get("Location")
	assert.Equal(t, "/auth", loc, "unauthenticated protected route should redirect to /auth")
}

// TestRedirectFollowsToLoginPage issues the redirect and follows it, proving
// the public /auth page is reachable from the redirect target and renders 200.
func TestRedirectFollowsToLoginPage(t *testing.T) {
	handler := newServerTestHandler(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Default client follows redirects.
	resp, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "login", "followed redirect should land on the login page")
}

// TestStaticAssetServed verifies the static-asset handler returns 200 for a
// request under /static.
func TestStaticAssetServed(t *testing.T) {
	handler := newServerTestHandler(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/static/css/main.css")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "color")
	assert.Equal(t, "text/css", resp.Header.Get("Content-Type"))
}

// TestUnknownRouteReturns404 verifies Chi's default NotFound behaviour for
// a path that no extension registered.
func TestUnknownRouteReturns404(t *testing.T) {
	handler := newServerTestHandler(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/this-route-does-not-exist")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestCookiePersistenceWithSession exercises the full cookie round-trip: an
// authentication-style middleware that recognises a session cookie, sets the
// user on the request context, and re-emits the cookie; then a follow-up
// request carrying that cookie reaches the protected handler. This is the
// Tier 4 cookie-persistence scenario without requiring a real database.
func TestCookiePersistenceWithSession(t *testing.T) {
	const sessionCookie = "fake-session-token-for-test"
	testUser := &model.User{ID: uuid.New(), Username: "alice", Role: model.RoleAdmin, Enabled: true}

	// sessionMiddleware recognises the oss_session cookie and populates the
	// request user, mirroring filter.Session + SessionRealm. On the protected
	// route this lets RequirePermission pass. It also sets a Set-Cookie on
	// responses when the user was recognised, mirroring session refresh.
	sessionMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie(web.SessionCookieName); err == nil && c.Value == sessionCookie {
				r = web.WithUser(r, testUser)
				http.SetCookie(w, web.CreateSessionCookie(sessionCookie, false))
			}
			next.ServeHTTP(w, r)
		})
	}

	permissionResolver := security.NewRoleBasedPermissionResolver()
	h, err := extplatform.NewHandler(extplatform.Options{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{stubServerExtension{}},
		MiddlewareChain: []func(http.Handler) http.Handler{
			sessionMiddleware,
		},
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			extplatform.GroupProtectedUI: {filter.RequirePermission(permissionResolver, "*", "*")},
		},
	})
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	defer srv.Close()

	// 1. Without a cookie, / redirects to /auth.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "unauthenticated should redirect")

	// 2. With a valid session cookie, / is served (the middleware populates the
	// user, so RequirePermission passes and the home handler renders).
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: web.SessionCookieName, Value: sessionCookie})
	authResp, err := client.Do(req)
	require.NoError(t, err)
	defer authResp.Body.Close()
	body, _ := io.ReadAll(authResp.Body)

	assert.Equal(t, http.StatusOK, authResp.StatusCode, "authenticated request should reach the home handler")
	assert.Contains(t, string(body), "alice", "home should render for the authenticated user")

	// 3. The session middleware re-emits the cookie on authenticated responses
	// (Set-Cookie present), proving cookie persistence.
	setCookies := authResp.Header.Values("Set-Cookie")
	require.NotEmpty(t, setCookies, "authenticated response should refresh the session cookie")
	found := false
	for _, c := range setCookies {
		if strings.HasPrefix(c, web.SessionCookieName+"=") {
			found = true
			break
		}
	}
	assert.True(t, found, "Set-Cookie should include the session cookie")

	// 4. The home handler receives the same user context on subsequent
	// requests (persistence across calls). Re-do the request via the default
	// client's cookie jar to prove the round-trip works as a browser would.
	jarClient := &http.Client{
		Jar: newMemoryCookieJar(),
	}
	// Seed the jar with the session cookie (as a login would).
	jarClient.Jar.SetCookies(mustParseURL(srv.URL), []*http.Cookie{
		{Name: web.SessionCookieName, Value: sessionCookie},
	})
	resp2, err := jarClient.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Contains(t, string(body2), "alice")
}

// mustParseURL parses u or fails the test.
func mustParseURL(u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return parsed
}

// memoryCookieJar is a minimal net/http.CookieJar implementation backed by a
// map, sufficient for the cookie-round-trip test. Each (scheme, host) gets its
// own cookie slice.
type memoryCookieJar struct {
	cookies map[string][]*http.Cookie
}

func newMemoryCookieJar() *memoryCookieJar {
	return &memoryCookieJar{cookies: map[string][]*http.Cookie{}}
}

func (j *memoryCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	key := u.Scheme + "://" + u.Host
	j.cookies[key] = append(j.cookies[key], cookies...)
}

func (j *memoryCookieJar) Cookies(u *url.URL) []*http.Cookie {
	key := u.Scheme + "://" + u.Host
	out := []*http.Cookie{}
	seen := map[string]bool{}
	// Merge any previously stored cookies, last-write-wins by Name.
	all := append([]*http.Cookie{}, j.cookies[key]...)
	for _, c := range all {
		if seen[c.Name] {
			continue
		}
		out = append(out, c)
		seen[c.Name] = true
	}
	return out
}
