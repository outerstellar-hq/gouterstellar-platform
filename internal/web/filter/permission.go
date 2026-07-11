package filter

import (
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

// RequirePermission returns middleware that checks the user has the given
// permission. If no user is set (unauthenticated), it redirects to /auth for
// browser routes or returns 401 for API routes. Authenticated but
// unauthorized users receive 403.
func RequirePermission(resolver security.PermissionResolver, domain, action string) func(http.Handler) http.Handler {
	perm := model.Permission{Domain: domain, Action: action, Instance: "*"}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := web.UserFromRequest(r)
			if user == nil {
				if isAPIRequest(r) {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				} else {
					http.Redirect(w, r, "/auth", http.StatusSeeOther)
				}
				return
			}
			if !resolver.Allowed(user, perm) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAPIRequest(r *http.Request) bool {
	return len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api"
}
