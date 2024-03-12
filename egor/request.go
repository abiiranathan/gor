package egor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

func SetContextValue(req *http.Request, key any, value interface{}) {
	ctx := context.WithValue(req.Context(), key, value)
	*req = *req.WithContext(ctx)

	// Set the value in the locals map. This allows us to access the value in the templates
	localCtx := req.Context().Value(contextKey).(*CTX)
	localCtx.Set(key, value)
}

func GetContextValue(req *http.Request, key any) interface{} {
	// get value from locals
	localCtx := req.Context().Value(contextKey).(*CTX)
	return localCtx.Get(key)
}

func SendJSON(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", ContentTypeJSON)
	return json.NewEncoder(w).Encode(v)
}

func SendHTML(w http.ResponseWriter, html string) error {
	w.Header().Set("Content-Type", ContentTypeHTML)
	_, err := w.Write([]byte(html))
	return err
}

func SendFile(w http.ResponseWriter, req *http.Request, file string) {
	http.ServeFile(w, req, file)
}

func SendString(w http.ResponseWriter, s string) error {
	w.Header().Set("Content-Type", ContentTypeText)
	_, err := w.Write([]byte(s))
	return err
}

// Sends the error message as a html string with the status code
func SendError(w http.ResponseWriter, err error, status ...int) {
	var statusCode = http.StatusInternalServerError
	if len(status) > 0 {
		statusCode = status[0]
	}
	w.Header().Set("Content-Type", ContentTypeHTML)
	w.WriteHeader(statusCode)
	_, err = w.Write([]byte(err.Error()))
	if err != nil {
		log.Println(err)
	}
}

// sends the error message as a JSON string with the status code
func SendJSONError(w http.ResponseWriter, key, s string, status ...int) {
	var statusCode = http.StatusInternalServerError
	if len(status) > 0 {
		statusCode = status[0]
	}

	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(fmt.Sprintf(`{"%s":"%s"}`, key, s))) // json.Encoder appends a newline
	if err != nil {
		log.Println(err)
	}
}

func GetContentType(req *http.Request) string {
	// content-type may contain additional information like charset
	ct := req.Header.Get("Content-Type")
	parts := strings.Split(ct, ";")
	return parts[0]
}

// Redirects the request to the given url.
// Default status code is 302 (http.StatusFound)
func Redirect(req *http.Request, w http.ResponseWriter, url string, status ...int) {
	var statusCode = http.StatusFound // Assume Afterpost request.
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
