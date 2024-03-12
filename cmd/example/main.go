package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/abiiranathan/egor/egor"
	"github.com/abiiranathan/egor/egor/middleware"
)

//go:embed static
var static embed.FS

func main() {
	mux := egor.NewRouter()
	logger := middleware.NewLogger(os.Stderr)

	mux.Use(middleware.Recover(true))
	mux.Use(logger.Logger)
	mux.Use(middleware.Etag())
	mux.Use(middleware.Cors())

	mux.StaticFS("/static", http.FS(static))

	mux.Get("/test/{id}", func(w http.ResponseWriter, r *http.Request) {
		egor.Redirect(w, r, "/redirect")

		// id := r.PathValue("id")
		// fmt.Fprintf(w, "Hello, you lucky number is %s!\n", id)
	})

	mux.Get("/redirect", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Redirected")
	})

	opts := []egor.ServerOption{
		egor.WithReadTimeout(time.Second * 10),
		egor.WithWriteTimeout(time.Second * 15),
	}

	server := egor.NewServer(":8080", mux, opts...)
	defer server.GracefulShutdown()
	server.ListenAndServe()
}
