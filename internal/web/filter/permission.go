package filter

import (
	"net/http"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

// RequirePermission returns middleware that checks the user has the given
// permission. If no user is set (unauthenticated), the response shape is chosen
// by wantsJSON: a JSON 401 for API/programmatic clients, or a redirect to /auth
// for browser clients. Authenticated but unauthorized users receive 403.
func RequirePermission(resolver security.PermissionResolver, domain, action string) func(http.Handler) http.Handler {
	perm := model.Permission{Domain: domain, Action: action, Instance: "*"}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := web.UserFromRequest(r)
			if user == nil {
				if wantsJSON(r) {
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				} else {
					http.Redirect(w, r, "/auth", http.StatusSeeOther)
				}
				return
			}
			if !resolver.Allowed(user, perm) {
				if wantsJSON(r) {
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				} else {
					http.Error(w, "Forbidden", http.StatusForbidden)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// wantsJSON reports whether the client expects a JSON response. It inspects the
// Accept header (application/json) and, as a fallback for clients that set
// Authorization but not Accept, treats Bearer-authenticated requests as JSON.
// This replaces the prior r.URL.Path[:4] == "/api" check, which panicked on
// paths shorter than 4 characters and conflated URL shape with content
// negotiation.
func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	v := r.Header.Get("Authorization")
	const prefix = "bearer "
	return len(v) > len(prefix) && strings.EqualFold(strings.TrimSpace(v[:len(prefix)]), "bearer")
}
