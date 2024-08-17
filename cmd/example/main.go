package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/abiiranathan/gor/gor"
	"github.com/abiiranathan/gor/gor/middleware/cors"
	"github.com/abiiranathan/gor/gor/middleware/csrf"
	"github.com/abiiranathan/gor/gor/middleware/etag"
	"github.com/abiiranathan/gor/gor/middleware/logger"
	"github.com/abiiranathan/gor/gor/middleware/recovery"
	"github.com/gorilla/sessions"
)

//go:embed static/*
var static embed.FS

func main() {
	t, err := gor.ParseTemplatesRecursiveFS(static, "static", template.FuncMap{})
	if err != nil {
		panic(err)
	}

	// Create a new router
	gor.NoTrailingSlash = true
	mux := gor.NewRouter(
		gor.WithTemplates(t),
		gor.PassContextToViews(true),
	)

	mux.Use(recovery.New(true))
	mux.Use(logger.New(os.Stderr, logger.StdLogFlags))
	mux.Use(etag.New())
	mux.Use(cors.New())

	// Create a cookie store.
	var store = sessions.NewCookieStore([]byte("secret key"))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   0,
		Domain:   "localhost",
		Secure:   false,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	mux.Use(csrf.New(store))
	mux.StaticFS("/static", http.FS(static))
	// mux.Static("/static/", "static")

	mux.Get("/test/{id}/", func(w http.ResponseWriter, r *http.Request) {
		gor.Redirect(w, r, "/redirect")

		// id := r.PathValue("id")
		// fmt.Fprintf(w, "Hello, you lucky number is %s!\n", id)
	})

	mux.Get("/redirect", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Redirected")
	})

	mux.Get("/api", func(w http.ResponseWriter, r *http.Request) {
		gor.SendJSONError(w, map[string]any{"error": "This is an error"}, http.StatusBadRequest)
	})

	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		gor.Render(w, r, "index.html", gor.Map{})
	})

	mux.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// log the csrf token
		fmt.Println(r.FormValue("csrf_token"))

		fmt.Fprintf(w, "Username: %s, Password: %s", username, password)
	})

	mux.FaviconFS(http.FS(static), "static/favicon.ico")

	opts := []gor.ServerOption{
		gor.WithReadTimeout(time.Second * 10),
		gor.WithWriteTimeout(time.Second * 15),
	}

	server := gor.NewServer(":8000", mux, opts...)
	defer server.Shutdown()

	log.Printf("Listening on %v\n", server.Addr)
	log.Fatalln(server.ListenAndServe())
}
