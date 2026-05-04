package filter

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type responseCapture struct {
	http.ResponseWriter
	status int
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New().String()[:8]
			ctx := context.WithValue(r.Context(), web.ContextKey("requestId"), requestID)
			r = r.WithContext(ctx)

			start := time.Now()
			rc := &responseCapture{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rc, r)

			duration := time.Since(start)

			slog.Info("request",
				"requestId", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rc.status,
				"duration", duration.String(),
				"remoteAddr", r.RemoteAddr,
			)
		})
	}
}
