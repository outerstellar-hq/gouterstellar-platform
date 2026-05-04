package filter

import (
	"context"
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type SessionLookup interface {
	LookupSession(ctx context.Context, rawToken string) model.SessionLookup
}

func Session(service SessionLookup, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := web.GetSessionToken(r)
			if rawToken == "" {
				next.ServeHTTP(w, r)
				return
			}

			result := service.LookupSession(r.Context(), rawToken)

			switch v := result.(type) {
			case model.SessionActive:
				r = web.WithUser(r, v.User)
				http.SetCookie(w, web.CreateSessionCookie(rawToken, secure))
			case model.SessionExpired:
				http.SetCookie(w, web.ClearSessionCookie(secure))
				http.Error(w, "Session expired", http.StatusUnauthorized)
				return
			case model.SessionNotFound:
			}

			next.ServeHTTP(w, r)
		})
	}
}
