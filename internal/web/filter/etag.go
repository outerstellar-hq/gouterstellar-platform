package filter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

type etagResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newETagResponse() *etagResponse {
	return &etagResponse{header: make(http.Header)}
}

func (e *etagResponse) Header() http.Header { return e.header }

func (e *etagResponse) WriteHeader(status int) {
	if e.status == 0 {
		e.status = status
	}
}

func (e *etagResponse) Write(body []byte) (int, error) {
	if e.status == 0 {
		e.status = http.StatusOK
	}
	return e.body.Write(body)
}

// ETag adds strong body-based validators to successful GET responses. Mount it
// on immutable or otherwise cacheable resources rather than application JSON.
func ETag() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.Header.Get("Range") != "" {
				next.ServeHTTP(w, r)
				return
			}

			response := newETagResponse()
			next.ServeHTTP(response, r)
			status := response.status
			if status == 0 {
				status = http.StatusOK
			}

			copyHeaders(w.Header(), response.header)
			if status != http.StatusOK {
				w.WriteHeader(status)
				_, _ = w.Write(response.body.Bytes())
				return
			}

			hash := sha256.Sum256(response.body.Bytes())
			etag := `"` + hex.EncodeToString(hash[:]) + `"`
			w.Header().Set("ETag", etag)
			if r.Header.Get("If-None-Match") == etag {
				w.Header().Del("Content-Length")
				w.WriteHeader(http.StatusNotModified)
				return
			}

			w.WriteHeader(status)
			_, _ = w.Write(response.body.Bytes())
		})
	}
}

func copyHeaders(destination, source http.Header) {
	for key, values := range source {
		destination[key] = append([]string(nil), values...)
	}
}
