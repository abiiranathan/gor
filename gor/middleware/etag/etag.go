package etag

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"

	"github.com/abiiranathan/gor/gor"
)

type etagResponseWriter struct {
	http.ResponseWriter              // the original ResponseWriter
	buf                 bytes.Buffer // buffer to store the response body
	hash                hash.Hash    // hash to calculate the ETag
	w                   io.Writer    // multiwriter to write to both the buffer and the hash
	status              int          // status code of the response
	written             bool         // whether the header has been written
}

func (e *etagResponseWriter) WriteHeader(code int) {
	e.status = code
	e.written = true
	// Don't actually write the header yet, we'll do that later
}

func (e *etagResponseWriter) Write(p []byte) (int, error) {
	if !e.written {
		// If WriteHeader was not explicitly called, we need to set the status
		e.status = http.StatusOK
		e.written = true
	}
	return e.w.Write(p)
}

func (e *etagResponseWriter) Flush() {
	if f, ok := e.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (e *etagResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := e.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func New(skip ...func(r *http.Request) bool) gor.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var skipEtag bool
			for _, s := range skip {
				if s(r) {
					skipEtag = true
					break
				}
			}

			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				skipEtag = true
			}

			if skipEtag {
				next.ServeHTTP(w, r)
				return
			}

			ew := &etagResponseWriter{
				ResponseWriter: w,
				buf:            bytes.Buffer{},
				hash:           sha1.New(),
				status:         http.StatusOK,
			}
			ew.w = io.MultiWriter(&ew.buf, ew.hash)

			next.ServeHTTP(ew, r)

			if ew.status != http.StatusOK {
				// For non-200 responses, write the status and body without ETag
				w.WriteHeader(ew.status)
				ew.buf.WriteTo(w)
				return
			}

			etag := fmt.Sprintf(`"%x"`, ew.hash.Sum(nil))
			w.Header().Set("ETag", etag)

			// Check If-None-Match and If-Match headers and return 304 or 412 if needed
			ifNoneMatch := r.Header.Get("If-None-Match")
			if ifNoneMatch == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			// If-Match is not supported for GET requests
			ifMatch := r.Header.Get("If-Match")
			if ifMatch != "" && ifMatch != etag {
				// If-Match header is present and doesn't match the ETag
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}

			// Write the status and body for 200 OK responses
			w.WriteHeader(ew.status)
			ew.buf.WriteTo(w)
		})
	}
}
