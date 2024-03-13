package main

import (
	"embed"
	"log"
	"net/http"
	"text/template"

	"github.com/abiiranathan/egor/egor"
	"github.com/gorilla/sessions"
)

//go:embed templates
var viewsFS embed.FS

// base.html is automatically added to every template.
// {{ .Content }} is replaced with page contents.
// No need for {{ template "base.html" . }} in every page.
func HomeHandler(w http.ResponseWriter, req *http.Request) {
	data := egor.Map{
		"Title": "Home Page",
		"Body":  "Welcome to the home page",
	}

	// Router is accessed in context and used for rending. Same as r.Render()
	// but this way you don't need r in scope.
	egor.Render(w, req, "home.html", data)
}

func AboutHandler(w http.ResponseWriter, req *http.Request) {
	data := egor.Map{
		"Title": "About Page",
		"Body":  "Welcome to the about page",
	}
	egor.Render(w, req, "about.html", data)
}

func NestedTemplate(w http.ResponseWriter, req *http.Request) {
	egor.Render(w, req, "doctor/doctor.html", egor.Map{})
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
			Title:     "Adding route groups in egor",
			Completed: false,
			Author:    "Abiira Nathan",
		},
	}

	egor.SendJSON(w, todos)
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
	egor.Render(w, req, "login.html", egor.Map{})
}

func main() {
	templ, err := egor.ParseTemplatesRecursiveFS(viewsFS, "templates", template.FuncMap{}, ".html")
	if err != nil {
		panic(err)
	}

	r := egor.NewRouter(
		egor.WithTemplates(templ),
		egor.PassContextToViews(true),
		egor.BaseLayout("base.html"),
		egor.ContentBlock("Content"),
	)

	r.Get("/{$}", HomeHandler)
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
		egor.SendString(w, "Hello "+username)
	})

	srv := egor.NewServer(":8080", r)

	log.Fatalln(srv.ListenAndServe())

}
