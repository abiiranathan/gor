# gor

Package gor (go router) implements a minimalistic but robust http router based on the standard go 1.22 enhanced routing capabilities in the `http.ServeMux`. It adds features like middleware support, helper methods for defining routes, template rendering with automatic template inheritance (of a base template).

It also has a BodyParser that decodes json, xml, url-encoded and multipart forms
based on content type. Form parsing supports all standard go types(and their pointers)
and slices of standard types. It also supports custom types that implement the `gor.FormScanner` interface.


## Installation

```bash
go get -u github.com/abiiranathan/gor
```

Example of a custom type that implements the FormScanner interface
```go
type FormScanner interface {
	FormScan(value interface{}) error
}

type Date time.Time // Date in format YYYY-MM-DD


// FormScan implements the FormScanner interface
func (d *Date) FormScan(value interface{}) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not a string")
	}

	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return fmt.Errorf("invalid date format")
	}
	*d = Date(t)
	return nil
}
```

gor supports single page application routing with a dedicated method `r.SPAHandler` that serves the index.html file for all routes that do not match a file or directory in the root directory of the SPA.

The router also supports route groups and subgroups with middleware that can be applied to the entire group or individual routes.


It has customizable built-in middleware for logging using the slog package, panic recovery, etag, cors, basic auth and jwt middlewares.

More middlewares can be added by implementing the Middleware type, a standard function that wraps an http.Handler. 

See the middleware package for examples.


```go
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

    "github.com/abiiranathan/gor/gor"
	"github.com/abiiranathan/gor/gor/middleware"
)

func main() {
    r := gor.NewRouter()
    r.Use(gor.Logger(os.Stdout))
    r.Use(gor.Recovery(true))
    r.Use(gor.Cors())
    r.Use(gor.ETag())

    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "Hello, World!")
    })

    r.Get("/hello/:name", func(w http.ResponseWriter, r *http.Request) {
        name := r.PathValue("name")
        fmt.Fprintf(w, "Hello, %s!", name)
        // r.SendString(fmt.Sprintf("Hello, %s!", name))
    })

    admin := r.Group("/admin", AdminRequired)
    admin.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "Admin Dashboard")
    })

    api := r.Group("/api")
    api.Get("/users", func(w http.ResponseWriter, r *http.Request) {
        users := []string{"John", "Doe", "Jane"}
        r.SendJSON(w, users)
    })

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
	gor.Render(w, req, "home.html", data)
}

func AboutHandler(w http.ResponseWriter, req *http.Request) {
	data := map[string]any{
		"Title": "About Page",
		"Body":  "Welcome to the about page",
	}
	gor.Render(w, req, "about.html", data)
}

func NestedTemplate(w http.ResponseWriter, req *http.Request) {
	gor.Render(w, req, "doctor/doctor.html", map[string]any{})
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
		gor.BaseLayout("base.html"),
		gor.ContentBlock("Content"),
	)

	r.Get("/", HomeHandler)
	r.Get("/about", AboutHandler)
	r.Get("/doctor", NestedTemplate)

	srv := gor.NewServer(":8080", r)
	log.Fatalln(srv.ListenAndServe())
}
```

No external libraries are included in the main package. Only a few external libraries are used in the middleware package.

## Helper functions at package level

- `func SetContextValue(req *http.Request, key any, value interface{})`
- `func GetContextValue(req *http.Request, key any) interface{}`
- `func SendJSON(w http.ResponseWriter, data interface{}) error`
- `func SendString(w http.ResponseWriter, data string) error`
- `func SendHTML(w http.ResponseWriter, html string) error`
- `func SendFile(w http.ResponseWriter, req *http.Request, file string)`
- `func SendError(w http.ResponseWriter, err error, status int)`
- `func SendJSONError(w http.ResponseWriter, key, s string, status int)`
- `func GetContentType(req *http.Request) string`
- `func Redirect(req *http.Request, w http.ResponseWriter, url string, status ...int)`
- `func Query(req *http.Request, key string, defaults ...string) string`
- `func QueryInt(req *http.Request, key string, defaults ...int) int`
- `func ParamInt(req *http.Request, key string, defaults ...int) int`
- `func SaveFile(fh *multipart.FileHeader, dst string) error`
- `func FormValue(req *http.Request, key string) string`
- `func FormData(req *http.Request) url.Values`
- `func FormFile(req *http.Request, key string) (*multipart.FileHeader, error)`
- `func FormFiles(req *http.Request, key string) ([]*multipart.FileHeader, error)`
- `func ParseMultipartForm(req *http.Request, maxMemory ...int64) (*multipart.Form, error)`
- `func BodyParser(req *http.Request, v interface{}) error`
- `func ExecuteTemplate(w io.Writer, req *http.Request, name string, data gor.Map) error`
- `ffunc LookupTemplate(req *http.Request, name string) (*template.Template, error)`
  
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
