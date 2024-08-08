package gor

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	ContentTypeJSON          string = "application/json"
	ContentTypeXML           string = "application/xml"
	ContentTypeXForm         string = "application/x-www-form-urlencoded"
	ContentTypeMultipartForm string = "multipart/form-data"
	ContentTypeHTML          string = "text/html"
	ContentTypeCSV           string = "text/csv"
	ContentTypeText          string = "text/plain"
	ContentTypeEventStream   string = "text/event-stream"
)

// Set a value in the request context. Also saves a copy in locals map.
// This allows for passing of passing of locals to templates.
func SetContextValue(req *http.Request, key any, value interface{}) {
	ctx := context.WithValue(req.Context(), key, value)
	*req = *req.WithContext(ctx)

	// Set the value in the locals map. This allows us to access the value in the templates
	if localCtx, ok := req.Context().Value(contextKey).(*CTX); ok {
		localCtx.Set(key, value)
	}
}

// return a value from context.
func GetContextValue(req *http.Request, key any) interface{} {
	// We don't use locals incase someone is using a different router.
	return req.Context().Value(key)
}

// Send v as JSON. Uses json.NewEncoder and sets content-type
// application/json for the response.
func SendJSON(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", ContentTypeJSON)
	return json.NewEncoder(w).Encode(v)
}

// Send HTML string.
func SendHTML(w http.ResponseWriter, html string) error {
	w.Header().Set("Content-Type", ContentTypeHTML)
	_, err := w.Write([]byte(html))
	return err
}

// Wrapper around http.Servefile.
func SendFile(w http.ResponseWriter, req *http.Request, file string) {
	http.ServeFile(w, req, file)
}

// Send string back to client.
func SendString(w http.ResponseWriter, s string) error {
	w.Header().Set("Content-Type", ContentTypeText)
	_, err := w.Write([]byte(s))
	return err
}

// Sends the error message to the client as html.
// If the Router has errorTemplate configured, the error template will be rendered instead.
// You can also pass a status code to be used.
// Yo do not need to call SendError after template rendering since the template will be rendered
// automatically if an error occurs during template rendering.
func SendError(w http.ResponseWriter, req *http.Request, err error, status ...int) {
	var statusCode = http.StatusInternalServerError
	if len(status) > 0 {
		statusCode = status[0]
	}

	// We are using go router.
	if writer, ok := w.(*ResponseWriter); ok {
		// get the CTX from the request
		ctx := req.Context().Value(contextKey).(*CTX)
		if ctx.Router.errorTemplate != "" {
			ctx.Router.renderErrorTemplate(writer, err, statusCode)
			return
		}
	}

	w.Header().Set("Content-Type", ContentTypeHTML)
	w.WriteHeader(statusCode)
	w.Write([]byte(err.Error()))
}

// sends the error message as a JSON string with the status code
func SendJSONError(w http.ResponseWriter, resp map[string]any, status ...int) {
	var statusCode = http.StatusInternalServerError
	if len(status) > 0 {
		statusCode = status[0]
	}

	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// Returns the header content type.
func GetContentType(req *http.Request) string {
	// content-type may contain additional information like charset
	ct := req.Header.Get("Content-Type")
	parts := strings.Split(ct, ";")
	return parts[0]
}

// Redirects the request to the given url.
// Default status code is 303 (http.StatusSeeOther)
func Redirect(w http.ResponseWriter, req *http.Request, url string, status ...int) {
	var statusCode = http.StatusSeeOther // Assume Afterpost request.
	if len(status) > 0 {
		statusCode = status[0]
	}
	http.Redirect(w, req, url, statusCode)
}

func Query(req *http.Request, key string, defaults ...string) string {
	v := req.URL.Query().Get(key)
	if v == "" && len(defaults) > 0 {
		return defaults[0]
	}
	return v
}

// paramInt returns the value of the parameter as an integer
func ParamInt(req *http.Request, key string, defaults ...int) int {
	v := req.PathValue(key)
	if v == "" && len(defaults) > 0 {
		return defaults[0]
	}

	vInt, err := strconv.Atoi(v)
	if err != nil {
		if len(defaults) > 0 {
			return defaults[0]
		}
		return 0
	}
	return vInt
}

// queryInt returns the value of the query as an integer
func QueryInt(req *http.Request, key string, defaults ...int) int {
	v := Query(req, key)
	if v == "" && len(defaults) > 0 {
		return defaults[0]
	}

	vInt, err := strconv.Atoi(v)
	if err != nil {
		if len(defaults) > 0 {
			return defaults[0]
		}
		return 0
	}
	return vInt
}

// save file
func SaveFile(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}
