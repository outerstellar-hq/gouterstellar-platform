package handler

import (
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

func requireAdminAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := web.UserFromRequest(r)
		if user == nil {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		if user.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
