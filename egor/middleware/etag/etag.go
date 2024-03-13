package etag

import (
	"bytes"
	"crypto/sha1"

	"fmt"
	"hash"
	"io"
	"net/http"

	"github.com/abiiranathan/egor/egor"
)

// etagResponseWriter is a wrapper around http.ResponseWriter that calculates the ETag
type etagResponseWriter struct {
	http.ResponseWriter              // Embedded to satisfy the interface
	buf                 bytes.Buffer // Buffer to store the response
	hash                hash.Hash    // Hash to calculate the ETag
	w                   io.Writer    // MultiWriter (buf, hash)
}

// Writes the response to the buffer and the hash
func (e *etagResponseWriter) Write(p []byte) (int, error) {
	return e.w.Write(p)
}

// New creates a new middleware handler that generates ETags for the response
// and validates the If-Match and If-None-Match headers.
func New(skip ...func(r *http.Request) bool) egor.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var skipEtag bool
			// Skip the middleware if the request matches the skip conditions
			for _, s := range skip {
				if s(r) {
					skipEtag = true
					break
				}
			}

			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				skipEtag = true
			}

			// Skip the middleware if the request matches the skip conditions
			if skipEtag {
				next.ServeHTTP(w, r)
				return
			}

			ew := &etagResponseWriter{
				ResponseWriter: w,
				buf:            bytes.Buffer{},
				hash:           sha1.New(),
			}

			ew.w = io.MultiWriter(&ew.buf, ew.hash)

			// Call the next handler
			next.ServeHTTP(ew, r)

			rw := w.(*egor.ResponseWriter)
			if rw.Status() != http.StatusOK {
				return // Don't generate ETags for invalid responses
			}

			etag := fmt.Sprintf("%x", ew.hash.Sum(nil))
			w.Header().Set("ETag", etag)

			// If-Match header validation.
			ifMatch := r.Header.Get("If-Match")
			if ifMatch != "" && ifMatch != etag {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}

			// If-None-Match header validation.
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(304)
			} else {
				// Write the buffer to the original response writer
				ew.buf.WriteTo(w)
			}
		})
	}
}
