package web

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

type ContextKey string

const (
	userContextKey     ContextKey = "user"
	csrfContextKey     ContextKey = "csrfToken"
	navItemsContextKey ContextKey = "navItems"
	shellChromeKey     ContextKey = "shellChrome"
	bannerLoaderKey    ContextKey = "bannerLoader"
	cspNonceKey        ContextKey = "cspNonce"
)

type ShellChrome struct {
	ShowSearchForm       bool
	ShowNotificationBell bool
}

// BannerLoader resolves extension-contributed notices only when a shared shell
// is rendered, avoiding remote provider work on JSON and asset requests.
type BannerLoader func(context.Context) ([]viewmodel.Banner, error)

func WithUser(r *http.Request, user *model.User) *http.Request {
	// Populate both the web-layer key (for renderer/handler use) and the
	// service-layer key (so services can derive the actor for side effects such
	// as notifications without importing the web package).
	ctx := context.WithValue(r.Context(), userContextKey, user)
	ctx = service.ContextWithUser(ctx, user)
	return r.WithContext(ctx)
}

func UserFromRequest(r *http.Request) *model.User {
	u, _ := r.Context().Value(userContextKey).(*model.User)
	return u
}

func WithCSRFToken(r *http.Request, token string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), csrfContextKey, token))
}

func CSRFTokenFromRequest(r *http.Request) string {
	t, _ := r.Context().Value(csrfContextKey).(string)
	return t
}

func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKey("requestId")).(string); ok {
		return id
	}
	return uuid.New().String()[:8]
}

// WithNavItems returns a new request with the given nav items stored in its
// context. The platform handler assembly injects the collected extension nav
// items here so the renderer can read them at request time.
func WithNavItems(r *http.Request, items []viewmodel.NavItem) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), navItemsContextKey, items))
}

// NavItemsFromContext returns the extension-contributed nav items stored in
// the context, or nil if none were set.
func NavItemsFromContext(ctx context.Context) []viewmodel.NavItem {
	items, _ := ctx.Value(navItemsContextKey).([]viewmodel.NavItem)
	return items
}

func WithShellChrome(r *http.Request, chrome ShellChrome) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), shellChromeKey, chrome))
}

func ShellChromeFromContext(ctx context.Context) (ShellChrome, bool) {
	chrome, ok := ctx.Value(shellChromeKey).(ShellChrome)
	return chrome, ok
}

func WithBannerLoader(r *http.Request, loader BannerLoader) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), bannerLoaderKey, loader))
}

func BannerLoaderFromContext(ctx context.Context) BannerLoader {
	loader, _ := ctx.Value(bannerLoaderKey).(BannerLoader)
	return loader
}

func WithCSPNonce(r *http.Request, nonce string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), cspNonceKey, nonce))
}

func CSPNonceFromRequest(r *http.Request) string {
	nonce, _ := r.Context().Value(cspNonceKey).(string)
	return nonce
}
