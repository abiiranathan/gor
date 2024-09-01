package main

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/abiiranathan/gor/gor"
)

//go:embed views
var viewsfs embed.FS

func main() {
	t, err := gor.ParseTemplatesRecursiveFS(viewsfs, "views", template.FuncMap{})
	if err != nil {
		panic(err)
	}

	// Working with components
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		err = t.ExecuteTemplate(w, "views/signup.html", map[string]any{
			"Username": "nathan",
			"Password": "password",
			"Email":    "myemail@gmail.com",
			"IsAdmin":  true,
			"Roles":    []string{"Admin", "Manager", "Supervisor"},
			"Age":      100,
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.ListenAndServe(":8000", nil)

}
