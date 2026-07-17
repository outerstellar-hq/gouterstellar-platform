package filter

import (
	"net/http"
	"strconv"
)

// MaxBodySize rejects declared oversized requests before a downstream handler
// reads or allocates the body. Requests without a valid declared length remain
// handler-owned, matching net/http streaming semantics.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			declared := r.ContentLength
			if declared < 0 {
				if parsed, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64); err == nil {
					declared = parsed
				}
			}
			if declared > maxBytes {
				http.Error(w, "Request body exceeds the limit.", http.StatusRequestEntityTooLarge)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
