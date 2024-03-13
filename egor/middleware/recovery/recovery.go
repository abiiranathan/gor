package recovery

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/abiiranathan/egor/egor"
)

// Panic recovery middleware.
// If stack trace is true, a stack trace will be logged.
// If errorHandler is passed, it will be called with the error. No response will be sent to the client.
// Otherwise the error will be logged and sent with a 500 status code.
func New(stackTrace bool, errorHandler ...func(err error)) egor.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						// must be a string
						err = errors.New(r.(string))
					}

					if len(errorHandler) > 0 {
						errorHandler[0](err)
					} else {
						log.Println(err)
						if stackTrace {
							log.Println(string(debug.Stack()))
						}

						w.WriteHeader(http.StatusInternalServerError)
						_, err = w.Write([]byte(err.Error()))
						if err != nil {
							log.Printf("could not write response: %v\n", err)
						}
					}

				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}
