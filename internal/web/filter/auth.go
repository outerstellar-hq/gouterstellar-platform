package filter

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/security"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

// AuthMetrics holds the Prometheus counters that observe bearer-token
// authentication outcomes.
type AuthMetrics struct {
	// Attempts counts authentication attempts labelled by realm and outcome
	// (e.g. "session"/"authenticated", "jwt"/"expired", "none"/"skipped").
	Attempts *prometheus.CounterVec
}

// NewAuthMetrics constructs an AuthMetrics and registers its counters with the
// supplied registry. The platform uses a custom (non-default) registry, so the
// counters must be registered explicitly here.
func NewAuthMetrics(reg *prometheus.Registry) *AuthMetrics {
	m := &AuthMetrics{
		Attempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_attempts_total",
			Help: "Total authentication attempts by realm and outcome",
		}, []string{"realm", "outcome"}),
	}
	reg.MustRegister(m.Attempts)
	return m
}

// BearerAuth returns middleware that attempts bearer-token authentication
// against each supplied realm (in order). Authentication outcomes are recorded
// on metrics. When no Authorization header is present or it is malformed, the
// request proceeds unauthenticated and no attempt is recorded.
func BearerAuth(metrics *AuthMetrics, realms ...security.AuthRealm) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]

			for _, realm := range realms {
				result := realm.Authenticate(r.Context(), token)
				switch v := result.(type) {
				case security.AuthenticatedResult:
					metrics.Attempts.WithLabelValues(realm.Name(), "authenticated").Inc()
					r = web.WithUser(r, v.User)
					next.ServeHTTP(w, r)
					return
				case security.ExpiredResult:
					metrics.Attempts.WithLabelValues(realm.Name(), "expired").Inc()
					w.Header().Set(SessionExpiredHeader, "true")
					http.Error(w, "Token expired", http.StatusUnauthorized)
					return
				case security.SkippedResult:
					metrics.Attempts.WithLabelValues(realm.Name(), "skipped").Inc()
				}
			}

			// No realm authenticated the token.
			metrics.Attempts.WithLabelValues("none", "skipped").Inc()
			next.ServeHTTP(w, r)
		})
	}
}
