package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/abiiranathan/egor/egor"
)

// Basic Auth middleware.
// If the username and password are not correct, a 401 status code is sent.
// The realm is the realm to display in the login box. Default is "Restricted".
func BasicAuth(username, password string, realm ...string) egor.Middleware {
	defaultRealm := "Restricted"
	if len(realm) > 0 {
		defaultRealm = realm[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			user, pass, ok := req.BasicAuth()

			if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {

				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, defaultRealm))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}
