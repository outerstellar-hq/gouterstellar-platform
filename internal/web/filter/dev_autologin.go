package filter

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/google/uuid"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

func DevAutoLogin(devAdminID func() uuid.UUID, secSvc *service.SecurityService, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if web.UserFromRequest(r) != nil {
				next.ServeHTTP(w, r)
				return
			}
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				next.ServeHTTP(w, r)
				return
			}
			adminID := devAdminID()
			if adminID == uuid.Nil {
				next.ServeHTTP(w, r)
				return
			}
			token, err := secSvc.CreateSession(r.Context(), adminID)
			if err != nil {
				slog.Warn("Dev auto-login failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			cookie := web.CreateSessionCookie(token, false)
			http.SetCookie(w, cookie)
			r.AddCookie(cookie)
			slog.Debug("Dev auto-login", "host", host)
			next.ServeHTTP(w, r)
		})
	}
}
