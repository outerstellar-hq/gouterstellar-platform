package filter

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// ErrorHandler recovers panics and delegates the response to the application's
// themed error renderer. A nil renderer falls back to a plain 500 response.
func ErrorHandler(render func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					slog.Error(
						"panic recovered",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"stack", string(debug.Stack()),
					)
					recovered := fmt.Errorf("panic: %v", err)
					if render != nil {
						render(w, r, recovered)
						return
					}
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
