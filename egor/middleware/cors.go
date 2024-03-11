package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/abiiranathan/egor/egor"
)

// CORSOptions is the configuration for the CORS middleware.
type CORSOptions struct {
	AllowedOrigins   []string // Origins that are allowed in the request, default is all origins
	AllowedMethods   []string // Methods that are allowed in the request
	AllowedHeaders   []string // Headers that are allowed in the request
	ExposedHeaders   []string // Headers that are exposed to the client
	AllowCredentials bool     // Allow credentials like cookies, authorization headers
	MaxAge           int      // Max age in seconds to cache preflight request
	Allowwebsockets  bool     // Allow websockets
}

// Cors middleware.
// If the origin is not allowed, a 403 status code is sent.
func Cors(opts ...CORSOptions) egor.Middleware {
	var options = CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           5,
		Allowwebsockets:  false,
	}

	if len(opts) > 0 {
		options = opts[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")

			if len(options.AllowedOrigins) > 0 {
				allowed := false
				for _, v := range options.AllowedOrigins {
					if v == origin || v == "*" {
						allowed = true
						break
					}
				}

				if !allowed {
					http.Error(w, "Origin not allowed", http.StatusForbidden)
					return
				}
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)

			if len(options.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", joinStrings(options.AllowedMethods))
			}

			if len(options.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", joinStrings(options.AllowedHeaders))
			}

			if len(options.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", joinStrings(options.ExposedHeaders))
			}

			if options.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if options.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", options.MaxAge))
			}

			if options.Allowwebsockets {
				w.Header().Set("Access-Control-Allow-Websocket", "true")
			}

			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func joinStrings(s []string) string {
	return strings.Join(s, ", ")
}
