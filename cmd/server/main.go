package main

import (
	"embed"
	"log"
	"net/http"
	"text/template"

	"github.com/abiiranathan/gor/gor"
	"github.com/gorilla/sessions"
)

//go:embed templates
var viewsFS embed.FS

// base.html is automatically added to every template.
// {{ .Content }} is replaced with page contents.
// No need for {{ template "base.html" . }} in every page.
func HomeHandler(w http.ResponseWriter, req *http.Request) {
	data := gor.Map{
		"Title": "Home Page",
		"Body":  "Welcome to the home page",
	}

	// Router is accessed in context and used for rending. Same as r.Render()
	// but this way you don't need r in scope.
	gor.Render(w, req, "templates/home.html", data)
}

func AboutHandler(w http.ResponseWriter, req *http.Request) {
	data := gor.Map{
		"Title": "About Page",
		"Body":  "Welcome to the about page",
	}
	gor.Render(w, req, "templates/about.html", data)
}

func NestedTemplate(w http.ResponseWriter, req *http.Request) {
	gor.Render(w, req, "templates/doctor/doctor.html", gor.Map{})
}

func ApiHandler(w http.ResponseWriter, req *http.Request) {
	todos := []struct {
		Title     string
		Completed bool
		Author    string
	}{
		{
			Title:     "Working on my portfolio",
			Completed: true,
			Author:    "Abiira Nathan",
		},
		{
			Title:     "Adding route groups in gor",
			Completed: false,
			Author:    "Abiira Nathan",
		},
	}

	gor.SendJSON(w, todos)
}

// For more persistent sessions, use a database store.
// e.g https://github.com/antonlindstrom/pgstore
var store = sessions.NewCookieStore([]byte("secret"))

// Create a protected handler
func protectedHandler(w http.ResponseWriter, req *http.Request) {
	session, _ := store.Get(req, "session-name")
	if session.Values["authenticated"] != true {
		// send a 401 status code
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := session.Values["user"]
	w.Write([]byte("Hello " + name.(string)))
}

func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		session, _ := store.Get(req, "session-name")
		if session.Values["authenticated"] != true {
			// redirect to login
			http.Redirect(w, req, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func loginGetHandler(w http.ResponseWriter, req *http.Request) {
	gor.Render(w, req, "templates/login.html", gor.Map{})
}

func main() {
	templ, err := gor.ParseTemplatesRecursiveFS(viewsFS, "templates", template.FuncMap{}, ".html")
	if err != nil {
		panic(err)
	}

	r := gor.NewRouter(
		gor.WithTemplates(templ),
		gor.PassContextToViews(true),
		gor.BaseLayout("templates/base.html"),
		gor.ContentBlock("Content"),
	)

	r.Get("/", HomeHandler)
	r.Get("/about", AboutHandler)
	r.Get("/api", ApiHandler)
	r.Get("/doctor", NestedTemplate)

	// Create a basic auth middleware
	r.Get("/login", loginGetHandler)
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		var username, password string
		username = r.FormValue("username")
		password = r.FormValue("password")

		if username == "admin" && password == "admin" {
			session, _ := store.Get(r, "session-name")
			session.Values["authenticated"] = true
			session.Values["user"] = username
			session.Save(r, w)
			http.Redirect(w, r, "/protected", http.StatusSeeOther)
			return
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)

	})

	r.Get("/protected", protectedHandler, SessionMiddleware)

	r.Get("/users/{username}", func(w http.ResponseWriter, r *http.Request) {
		username := r.PathValue("username")
		gor.SendString(w, "Hello "+username)
	})

	srv := gor.NewServer(":8080", r)
	defer srv.Shutdown()

	log.Fatalln(srv.ListenAndServe())
}
