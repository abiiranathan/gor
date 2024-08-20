# gor

Package **gor** (go router) implements a minimalistic but robust http router based on the standard go 1.22 enhanced routing capabilities in the `http.ServeMux`. It adds features like middleware support, helper methods for defining routes, template rendering with automatic template inheritance (of a base template).

It also has a BodyParser that decodes json, xml, url-encoded and multipart forms
based on content type. Form parsing supports all standard go types(and their pointers)
and slices of standard types. 
It also supports custom types that implement the `gor.FormScanner` interface.

**gor** supports single page application routing with a dedicated method `r.SPAHandler` that serves the index.html file for all routes that do not match a file or directory in the root directory of the SPA.

The router also supports route groups and subgroups with middleware that can be applied to the entire group or individual routes.

It has customizable built-in middleware for logging using the slog package, panic recovery, etag, cors, basic auth and jwt middlewares.

More middlewares can be added by implementing the Middleware type, a standard function that wraps an http.Handler. 

See the middleware package for examples.

## Installation

```bash
go get -u github.com/abiiranathan/gor
```

### Example of a custom type that implements the FormScanner interface
```go
type FormScanner interface {
	FormScan(value interface{}) error
}

type Date time.Time // Date in format YYYY-MM-DD

// FormScan implements the FormScanner interface
func (d *Date) FormScan(value interface{}) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("date value is not a string")
	}

	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return fmt.Errorf("invalid date format")
	}
	*d = Date(t)
	return nil
}
```




## Rendering Templates
See the [example](./cmd/server/main.go) for a complete example of how to use the gor package.

```go

package main

import (
	"github.com/abiiranathan/gor/gor"
	"embed"
	"log"
	"net/http"
	"text/template"
)

//go:embed templates
var viewsFS embed.FS

// base.html is automatically added to every template.
// {{ .Content }} is replaced with page contents.
// No need for {{ template "base.html" . }} in every page.
func HomeHandler(w http.ResponseWriter, req *http.Request) {
	data := map[string]any{
		"Title": "Home Page",
		"Body":  "Welcome to the home page",
	}

	// Router is accessed in context and used for rending. Same as r.Render()
	// but this way you don't need r in scope.
	gor.Render(w, req, "templates/home.html", data)
}

func AboutHandler(w http.ResponseWriter, req *http.Request) {
	data := map[string]any{
		"Title": "About Page",
		"Body":  "Welcome to the about page",
	}
	gor.Render(w, req, "templates/about.html", data)
}

func NestedTemplate(w http.ResponseWriter, req *http.Request) {
	gor.Render(w, req, "templates/doctor/doctor.html", map[string]any{})
}


func main() {
	templ, err := gor.ParseTemplatesRecursiveFS(viewsFS, "templates", template.FuncMap{}, ".html")
	if err != nil {
		panic(err)
	}

    /*
    OR 
    templ, err := gor.ParseTemplatesRecursive(viewsDirname, template.FuncMap{}, ".html")
	if err != nil {
		panic(err)
	}
    */

	r := gor.NewRouter(
		gor.WithTemplates(templ),
		gor.PassContextToViews(true),
		gor.BaseLayout("templates/base.html"),
		gor.ContentBlock("Content"),
	)

	r.Get("/", HomeHandler)
	r.Get("/about", AboutHandler)
	r.Get("/doctor", NestedTemplate)

	srv := gor.NewServer(":8080", r)
	log.Fatalln(srv.ListenAndServe())
}
```

> Only a few external libraries are used in the middleware subpackage.

## Tests
    
```bash
go test -v ./...
```

## Benchmarks

```bash
go test -bench=. ./... -benchmem
```

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.
Please make sure to update tests as appropriate.

## License

[MIT](https://choosealicense.com/licenses/mit/)
