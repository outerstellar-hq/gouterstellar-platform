package filter

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

const (
	// RequestIDHeader carries the correlation ID shared by logs and clients.
	RequestIDHeader = "X-Request-Id"
	// SessionExpiredHeader lets API clients distinguish expiry from other 401 responses.
	SessionExpiredHeader = "X-Session-Expired"
)

type responseCapture struct {
	http.ResponseWriter
	status int
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rc.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	rc.status = http.StatusSwitchingProtocols
	return hijacker.Hijack()
}

func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := strings.TrimSpace(r.Header.Get(RequestIDHeader))
			if requestID == "" || len(requestID) > 128 {
				requestID = uuid.New().String()[:8]
			}
			ctx := context.WithValue(r.Context(), web.ContextKey("requestId"), requestID)
			r = r.WithContext(ctx)
			w.Header().Set(RequestIDHeader, requestID)

			start := time.Now()
			rc := &responseCapture{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rc, r)

			duration := time.Since(start)

			slog.Info(
				"request",
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
