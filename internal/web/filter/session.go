package filter

import (
	"context"
	"net/http"
	"strings"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
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
				if isWebSocketRequest(r) {
					next.ServeHTTP(w, r)
					return
				}
				http.SetCookie(w, web.ClearSessionCookie(secure))
				w.Header().Set(SessionExpiredHeader, "true")
				if isAPIPath(r.URL.Path) || wantsJSON(r) {
					http.Error(w, "Session expired", http.StatusUnauthorized)
				} else {
					http.Redirect(w, r, "/auth?expired=true", http.StatusFound)
				}
				return
			case model.SessionNotFound:
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}
