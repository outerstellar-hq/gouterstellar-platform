package filter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"net/http"
)

type etagResponse struct {
	http.ResponseWriter
	buf bytes.Buffer
}

func (e *etagResponse) Write(b []byte) (int, error) {
	return e.buf.Write(b)
}

func (e *etagResponse) WriteHeader(code int) {
	e.ResponseWriter.WriteHeader(code)
}

func ETag() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			er := &etagResponse{ResponseWriter: w}
			next.ServeHTTP(er, r)

			hash := sha256.Sum256(er.buf.Bytes())
			etag := `"` + hex.EncodeToString(hash[:]) + `"`

			ifNoneMatch := r.Header.Get("If-None-Match")
			if ifNoneMatch == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			w.Header().Set("Etag", etag)
			w.Write(er.buf.Bytes())
		})
	}
}
