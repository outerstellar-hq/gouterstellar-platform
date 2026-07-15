package filter

import (
	"net/http"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

func BearerAuth(realms ...security.AuthRealm) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
				return
			}

			token := parts[1]

			for _, realm := range realms {
				result := realm.Authenticate(token)
				switch v := result.(type) {
				case security.AuthenticatedResult:
					r = web.WithUser(r, v.User)
					next.ServeHTTP(w, r)
					return
				case security.ExpiredResult:
					http.Error(w, "Token expired", http.StatusUnauthorized)
					return
				case security.SkippedResult:
				}
			}

			http.Error(w, "Invalid token", http.StatusUnauthorized)
		})
	}
}
