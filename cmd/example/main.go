package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/abiiranathan/egor/egor"
	"github.com/abiiranathan/egor/egor/middleware/cors"
	"github.com/abiiranathan/egor/egor/middleware/csrf"
	"github.com/abiiranathan/egor/egor/middleware/etag"
	"github.com/abiiranathan/egor/egor/middleware/logger"
	"github.com/abiiranathan/egor/egor/middleware/recovery"
	"github.com/gorilla/sessions"
)

//go:embed static/*
var static embed.FS

func main() {
	t, err := egor.ParseTemplatesRecursiveFS(static, "static", template.FuncMap{})
	if err != nil {
		panic(err)
	}

	// Create a new router
	egor.NoTrailingSlash = true
	mux := egor.NewRouter(
		egor.WithTemplates(t),
		egor.PassContextToViews(true),
	)

	mux.Use(recovery.New(false))
	mux.Use(logger.New(os.Stderr).Logger)
	mux.Use(etag.New())
	mux.Use(cors.New())

	var store = sessions.NewCookieStore([]byte("secret key"))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   0,
		Domain:   "localhost",
		Secure:   false,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	mux.Use(csrf.New(store).Middleware)
	mux.StaticFS("/static", http.FS(static))
	// mux.Static("/static/", "static")

	mux.Get("/test/{id}/", func(w http.ResponseWriter, r *http.Request) {
		egor.Redirect(w, r, "/redirect")

		// id := r.PathValue("id")
		// fmt.Fprintf(w, "Hello, you lucky number is %s!\n", id)
	})

	mux.Get("/redirect", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Redirected")
	})

	mux.Get("/api", func(w http.ResponseWriter, r *http.Request) {
		egor.SendJSONError(w, "error", "This is an error", http.StatusBadRequest)

	})

	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		egor.Render(w, r, "index.html", egor.Map{})
	})

	mux.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// log the csrf token
		fmt.Println(r.FormValue("csrf_token"))

		fmt.Fprintf(w, "Username: %s, Password: %s", username, password)
	})

	mux.FaviconFS(http.FS(static), "static/favicon.ico")

	opts := []egor.ServerOption{
		egor.WithReadTimeout(time.Second * 10),
		egor.WithWriteTimeout(time.Second * 15),
	}

	server := egor.NewServer(":8080", mux, opts...)
	defer server.GracefulShutdown()
	server.ListenAndServe()
}
