/*
Package gor(enhanced go router) is a minimalistic, robust http router based on the go 1.22
enhanced routing capabilities. It adds a few features like middleware support, helper methods
for defining routes, template rendering with automatic template inheritance(of a layout template),
json,xml,form parsing based on content type, Single page application routing, grouping routes and more.

Has customizable built-in middleware for logging using the slog package, recovery, etag, cors and jwt middlewares.
More middlewares can be added by implementing the Middleware type, a standard function that wraps an http.Handler.

No external libraries are included in the main package. The only external library is the
middleware package which is optional.
*/
package gor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

var (
	// Match only the root path with "/" contrary to the default behavior which matches everything.
	// The default is true.
	StrictHome = true

	// Remove trailing slashes from the pattern (and req.URL.Path) except for the root path.
	// This means that if you register "/test/" and a request is made to "/test" or "/test/",
	// it will not match.
	// The default is true.
	NoTrailingSlash = true

	// name of the template content block
	contentBlock = "Content"
)

type contextType string

// Standard function that wraps an http.Handler.
type Middleware func(next http.Handler) http.Handler

// Generic type for any value used to pass data around between templates and context.
type Map map[string]any

// contextKey is the key used to store the custom CTX inside the request context.
// Access the context with
//
//	ctx := req.Context().Value(gor.contextKey).(*gor.CTX)
const contextKey = contextType("ctx")

type route struct {
	prefix      string       // contains the method and the path
	middlewares []Middleware // Middlewares
	handler     http.Handler // Route handler
}

// Router is a simple router that implements the http.Handler interface
type Router struct {
	globalMiddlewares []Middleware      // Global middlewares
	routes            map[string]*route // Routes mapped to their prefix
	mux               *http.ServeMux    // ServeMux

	// Configuration for templates

	viewsFs            fs.FS              // Views embed.FS(Alternative to views if set)
	template           *template.Template // All parsed templates
	baseLayout         string             // Base layout for the templates(default is "")
	contentBlock       string             // Content block for the templates(default is "Content")
	errorTemplate      string             // Error template. Passed "error", "status", "status_text" in its context.
	passContextToViews bool               // Pass the request context to the views

	// groups
	groups map[string]*Group // Groups mapped to their prefix

	// Handler for 404 not found errors. Note that when this is called,
	// The request parameters are not available, since they are populated by the http.ServeMux
	// when the request is matched to a route. So calling r.PathValue() will return "".
	NotFoundHandler http.Handler
}

// CTX is the custom context passed inside the request context.
// It carries a reference to the gor.Router and unexported fields
// for tracking locals.
//
// It can be access from context with:
//
//	ctx := req.Context().Value(gor.ContextKey).(*gor.CTX)
type CTX struct {
	context  context.Context // The request context
	localsMu *sync.RWMutex   // Mutex to syncronize access to the locals map
	locals   map[any]any     // Locals for the templates
	Router   *Router         // The router
}

type ResponseWriter struct {
	http.ResponseWriter     // The embedded response writer.
	status              int // response status code

	// track if status already sent
	statusSent bool
}

// WriteHeader sends an HTTP response header with the provided status code.
func (w *ResponseWriter) WriteHeader(status int) {
	if w.statusSent {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
	w.statusSent = true
}

// Status returns the response status code.
func (w *ResponseWriter) Status() int {
	return w.status
}

// Flush sends any buffered data to the client.
func (w *ResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Push initiates an HTTP/2 server push.
// See https://pkg.go.dev/net/http#Pusher.Push
func (w *ResponseWriter) Push(target string, opts *http.PushOptions) {
	if f, ok := w.ResponseWriter.(http.Pusher); ok {
		f.Push(target, opts)
	}
}

// Hijack lets the caller take over the connection.
// After a call to Hijack the HTTP server library will not do anything else with the connection.
// see https://pkg.go.dev/net/http#Hijacker.Hijack
func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("http.Hijacker is not implemented")
}

// Router option a function option for configuring the router.
type RouterOption func(*Router)

// NewRouter creates a new router with the given options.
// The router wraps the http.DefaultServeMux and adds routing and middleware
// capabilities.
func NewRouter(options ...RouterOption) *Router {
	r := &Router{
		mux:                http.NewServeMux(),
		routes:             make(map[string]*route),
		passContextToViews: false,
		baseLayout:         "",
		contentBlock:       contentBlock,
		viewsFs:            nil,
		groups:             make(map[string]*Group),
		globalMiddlewares:  []Middleware{},
		template:           nil,
	}

	for _, option := range options {
		option(r)
	}
	return r
}

// Apply a global middleware to all routes.
func (r *Router) Use(middlewares ...Middleware) {
	r.globalMiddlewares = append(r.globalMiddlewares, middlewares...)
}

var ctxPool = sync.Pool{
	New: func() interface{} {
		return &CTX{
			localsMu: &sync.RWMutex{},
			locals:   make(map[any]any),
		}
	},
}

// Implementation for http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// if no trailing slash is allowed, remove it
	if NoTrailingSlash && req.URL.Path != "/" {
		req.URL.Path = strings.TrimSuffix(req.URL.Path, "/")
	}

	writer := &ResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}

	// Get a context from the pool
	ctx := ctxPool.Get().(*CTX)
	ctx.context = req.Context()
	ctx.Router = r

	defer func() {
		// Reset the context
		ctx.context = nil
		ctx.Router = nil

		for k := range ctx.locals {
			delete(ctx.locals, k)
		}
		ctxPool.Put(ctx)
	}()

	// set the context
	valueContext := context.WithValue(req.Context(), contextKey, ctx)
	*req = *req.WithContext(valueContext)

	// Call the NotFoundHandler if no route is found
	_, pattern := r.mux.Handler(req)
	if pattern == "" {
		if r.NotFoundHandler != nil {
			r.NotFoundHandler.ServeHTTP(writer, req)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}

	r.mux.ServeHTTP(writer, req)
}

// chain of middlewares
func (r *Router) chain(middlewares []Middleware, handler http.Handler) http.Handler {
	if len(middlewares) == 0 {
		return handler
	}

	// wrap the handler with the last middleware
	wrapped := middlewares[len(middlewares)-1](handler)

	// wrap the handler with the remaining middlewares
	for i := len(middlewares) - 2; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

// Set a value to the context.
func (r *CTX) Set(key any, value any) {
	r.localsMu.Lock()
	defer r.localsMu.Unlock()
	r.locals[key] = value
}

// Get a value from the context.
func (r *CTX) Get(key any) any {
	r.localsMu.RLock()
	defer r.localsMu.RUnlock()
	return r.locals[key]
}

// registerRoute registers a route with the router.
func (r *Router) registerRoute(method, path string, handler http.HandlerFunc, middlewares []Middleware) {
	if StrictHome && path == "/" {
		path = path + "{$}" // Match only the root path
	}

	// remove trailing slashes
	if NoTrailingSlash && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}

	prefix := fmt.Sprintf("%s %s", method, path)

	// chain the route middlewares
	var h http.Handler
	h = r.chain(middlewares, handler)

	// chain the global middlewares
	h = r.chain(r.globalMiddlewares, h)

	newRoute := &route{prefix: prefix, middlewares: middlewares, handler: h}

	// add the route to the routes map
	r.routes[prefix] = newRoute

	r.mux.Handle(prefix, h)
}

// GET request.
func (r *Router) Get(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodGet, path, handler, middlewares)
}

// POST request.
func (r *Router) Post(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodPost, path, handler, middlewares)
}

// PUT request.
func (r *Router) Put(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodPut, path, handler, middlewares)
}

// PATCH request.
func (r *Router) Patch(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodPatch, path, handler, middlewares)
}

// DELETE request.
func (r *Router) Delete(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodDelete, path, handler, middlewares)
}

// OPTIONS. This may not be necessary as registering GET request automatically registers OPTIONS.
func (r *Router) Options(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodOptions, path, handler, middlewares)
}

// HEAD request.
func (r *Router) Head(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodHead, path, handler, middlewares)
}

// TRACE http request.
func (r *Router) Trace(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodTrace, path, handler, middlewares)
}

// CONNECT http request.
func (r *Router) Connect(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.registerRoute(http.MethodConnect, path, handler, middlewares)
}

// Serve static assests at prefix in the directory dir.
// e.g r.Static("/static", "static").
// This method will strip the prefix from the URL path.
// To serve minified assets(JS and CSS) if present, call gor.ServeMinifiedAssetsIfPresent=true.
func (r *Router) Static(prefix, dir string) {
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	var h = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := filepath.Join(dir, strings.TrimPrefix(req.URL.Path, prefix))

		if ServeMinifiedAssetsIfPresent {
			stat, err := os.Stat(path)
			if err != nil || stat.IsDir() {
				http.NotFound(w, req)
				return
			}

			if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
				// Check for the minified version of the file
				minifiedPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".min" + filepath.Ext(path)
				if filePathExists(minifiedPath) {
					http.ServeFile(w, req, minifiedPath)
					return
				}
			}
		}

		http.ServeFile(w, req, path)

	})

	r.mux.Handle(prefix, r.chain(r.globalMiddlewares, h))
}

func filePathExists(name string) bool {
	stat, err := os.Stat(name)
	return err == nil && !stat.IsDir()
}

// Wrapper around http.ServeFile.
func (r *Router) File(path, file string) {
	var hf http.HandlerFunc = func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, file)
	}

	handler := r.chain(r.globalMiddlewares, hf)
	r.Get(path, handler.ServeHTTP)
}

func (r *Router) FileFS(fs http.FileSystem, prefix, path string) {
	r.Get(prefix, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f, err := fs.Open(path)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, req)
			return
		}

		w.WriteHeader(http.StatusOK)
		http.ServeContent(w, req, path, stat.ModTime(), f)
	}))
}

// Serve favicon.ico from the file system fs at path.
func (r *Router) FaviconFS(fs http.FileSystem, path string) {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f, err := fs.Open(path)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, req)
			return
		}

		var data = make([]byte, stat.Size())
		_, err = f.Read(data)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		w.Header().Set("Content-Disposition", "inline; filename=favicon.ico")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})

	r.Get("/favicon.ico", handler)
}

type minifiedFS struct {
	http.FileSystem
}

func (mfs *minifiedFS) Open(name string) (http.File, error) {
	fmt.Printf("Serving %s\n", name)

	// Check if the requested file is a .js or .css file
	if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") {
		// Check for the minified version of the file
		minifiedName := strings.TrimSuffix(name, filepath.Ext(name)) + ".min" + filepath.Ext(name)

		// Return minified file if available.
		if f, err := mfs.FileSystem.Open(minifiedName); err == nil {
			return f, nil
		}
	}

	// If no minified version is found, serve the original file
	return mfs.FileSystem.Open(name)
}

// Serve minified Javascript and CSS if present instead of original file.
// This applies to StaticFS, Static functions.
// e.g /static/js/main.js will serve /static/js/main.min.js if present.
// Default is false.
// This is important since we maintain the same script sources in our templates/html.
var ServeMinifiedAssetsIfPresent = false

// Like Static but for http.FileSystem.
// Use this to serve embedded assets with go/embed.
//
//	mux.StaticFS("/static", http.FS(staticFs))
func (r *Router) StaticFS(prefix string, fs http.FileSystem) {
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	if ServeMinifiedAssetsIfPresent {
		fs = &minifiedFS{fs}
	}

	// Create file server for the http.FileSystem
	handler := http.FileServer(fs)

	// Apply global middleware
	finalHandler := r.chain(r.globalMiddlewares, handler)
	r.mux.Handle(prefix, finalHandler)
}

// creates a new http.FileSystem from the embed.FS
func buildFS(frontendFS fs.FS, root string) http.FileSystem {
	fsys, err := fs.Sub(frontendFS, root)
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}

// SPAOptions for customizing the cache control and index file.
type SPAOptions struct {
	CacheControl     string           // default is empty, example: "public, max-age=31536000"
	ResponseModifier http.HandlerFunc // allows fo modifying request/response
	Skip             []string         // skip these routes and return 404 if they match
	Index            string           // default is index.html
}

// Serves Single Page applications like svelte-kit, react etc.
// frontendFS is any interface that satisfies fs.FS, like embed.FS,
// http.Dir() wrapping a directory etc.
// path is the mount point: likely "/".
// buildPath is the path to build output containing your entry point html file.
// The default entrypoint is "index.html" i.e buildPath/index.html.
// You can change the entrypoint with options. Passed options override all defaults.
func (r *Router) SPAHandler(frontendFS fs.FS, path string, buildPath string, options ...SPAOptions) {
	var (
		indexFile    = "index.html"
		cacheControl string
		skip         []string
		resModifier  http.HandlerFunc = nil
	)

	if len(options) > 0 {
		option := options[0]

		cacheControl = option.CacheControl
		skip = option.Skip

		if option.Index != "" {
			indexFile = option.Index
		}
		resModifier = option.ResponseModifier
	}

	indexFp, err := frontendFS.Open(filepath.Join(buildPath, indexFile))
	if err != nil {
		panic(err)
	}

	index, err := io.ReadAll(indexFp)
	if err != nil {
		panic("Unable to read contents of " + indexFile)
	}

	// Apply global middleware
	fsHandler := http.FileServer(buildFS(frontendFS, buildPath))
	handler := r.chain(r.globalMiddlewares, fsHandler)

	r.mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		// check skip.
		for _, s := range skip {
			if s == req.URL.Path {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
		}

		baseName := filepath.Base(req.URL.Path)
		if req.URL.Path == "/" {
			baseName = indexFile
		}

		// open the file from the embed.FS
		f, err := frontendFS.Open(filepath.Join(buildPath, baseName))
		if err != nil {
			if os.IsNotExist(err) {
				// Could be an invalid API request
				if filepath.Ext(req.URL.Path) != "" {
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}

				// Send the html content type.
				w.Header().Set("Content-Type", ContentTypeHTML)

				// set cache control headers if specified by user.
				if cacheControl != "" {
					w.Header().Set("Cache-Control", cacheControl)
				}

				w.WriteHeader(http.StatusAccepted)

				// Allow user to modify response.
				if resModifier != nil {
					resModifier(w, req)
				}

				// send index.html
				w.Write(index)
			} else {
				// IO Error
				http.Error(w, "500 internal server error", http.StatusInternalServerError)
			}
			return
		} else {
			// we found the file, send it if not a directory.
			defer f.Close()
			stat, err := f.Stat()
			if err != nil {
				http.Error(w, "500 internal server error", http.StatusInternalServerError)
				return
			}

			if stat.IsDir() {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			// The file system handler knows how to serve JS/CSS and other assets with the correct
			// content type.
			handler.ServeHTTP(w, req)
		}
	})
}

// render error template
func (r *Router) renderErrorTemplate(w http.ResponseWriter, err error, status ...int) {
	var statusCode = http.StatusInternalServerError
	if len(status) > 0 {
		statusCode = status[0]
	}

	// send the error
	w.Header().Set("Content-Type", ContentTypeHTML)

	if r.errorTemplate != "" {
		err = r.renderTemplate(w, r.errorTemplate, Map{
			"status":      statusCode,
			"status_text": http.StatusText(statusCode),
			"error":       err,
		})
		if err != nil {
			log.Println(err)
		}
	} else {
		w.Write([]byte(err.Error()))
	}
	w.WriteHeader(statusCode)
}

func (r *Router) RenderError(w http.ResponseWriter, err error, status ...int) {
	r.renderErrorTemplate(w, err, status...)
}

// =========== TEMPLATE FUNCTIONS ===========
func (r *Router) renderTemplate(w io.Writer, name string, data Map) error {
	// if name is missing the extension, add it(assume it's an html file)
	if filepath.Ext(name) == "" {
		name = name + ".html"
	}

	buf := new(bytes.Buffer)
	err := r.template.ExecuteTemplate(buf, name, data)
	if err != nil {
		log.Printf("Error rendering template: %s\n", err)
		return err
	}

	content := buf.String()

	finalBuf := new(bytes.Buffer)
	data[r.contentBlock] = template.HTML(content)
	err = r.template.ExecuteTemplate(finalBuf, r.baseLayout, data)

	if err != nil {
		log.Printf("Error rendering template: %s\n", err)
		return err
	}

	if writer, ok := w.(http.ResponseWriter); ok {
		writer.Header().Set("Content-Type", ContentTypeHTML)
		writer.WriteHeader(http.StatusOK)
	}

	_, err = w.Write(finalBuf.Bytes())
	return err
}

// Render the template tmpl with the data. If no template is configured, Render will panic.
// data is a map such that it can be extended with
// the request context keys if passContextToViews is set to true.
// If a file extension is missing, it will be appended as ".html".
func (r *Router) Render(w io.Writer, req *http.Request, name string, data Map) {
	if r.template == nil {
		panic("No template is configured")
	}

	writeError := func(err error) {
		if err != nil {
			log.Println(err)
			if writer, ok := w.(http.ResponseWriter); ok {
				writer.Header().Set("Content-Type", ContentTypeHTML)
				writer.Write([]byte(err.Error()))
				writer.WriteHeader(http.StatusInternalServerError)
			}
		}
	}

	// pass the request context to the views
	if r.passContextToViews {
		ctx, ok := req.Context().Value(contextKey).(*CTX)
		if ok {
			for k, v := range ctx.locals {
				data[fmt.Sprintf("%v", k)] = v
			}
		}
	}

	// if baseLayout and contentBlock are set, render the template with the base layout
	if r.baseLayout != "" && r.contentBlock != "" {
		err := r.renderTemplate(w, name, data)
		writeError(err)
		return
	}

	err := r.template.ExecuteTemplate(w, name, data)
	writeError(err)

}

// Render a template of given name and pass the data to it.
// Make sure you are using gor.Router. Otherwise this function will panic.
// If a file extension is missing, it will be appended as ".html".
func Render(w io.Writer, req *http.Request, name string, data Map) {
	ctx, ok := req.Context().Value(contextKey).(*CTX)
	if !ok {
		panic("You are not using gor.Router. You cannot use this function")
	}
	ctx.Router.Render(w, req, name, data)
}

// Execute a standalone template without a layout.
func (r *Router) ExecuteTemplate(w io.Writer, name string, data Map) error {
	if r.template == nil {
		panic("No template is configured")
	}

	// append the file extension if missing
	if filepath.Ext(name) == "" {
		name = name + ".html"
	}
	return r.template.ExecuteTemplate(w, name, data)
}

// Execute a standalone template without a layout.
// To execute a named template-without inserting base layout, first call
// LookupTemplate and then execute it yourself using standard html/template
// semantics. If the extension is missing in name, .html is assumed.
func ExecuteTemplate(w io.Writer, req *http.Request, name string, data Map) error {
	ctx, ok := req.Context().Value(contextKey).(*CTX)
	if !ok {
		panic("You are not using gor.Router. You cannot use this function")
	}

	// append the file extension if missing
	if filepath.Ext(name) == "" {
		name = name + ".html"
	}

	return ctx.Router.ExecuteTemplate(w, name, data)
}

// Execute a standalone template without a layout.
// If the extension is missing in name, .html is assumed.
func LookupTemplate(req *http.Request, name string) (*template.Template, error) {
	ctx, ok := req.Context().Value(contextKey).(*CTX)
	if !ok {
		return nil, fmt.Errorf("you are not using gor.Router. You cannot use this function")
	}

	if ctx.Router.template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	t := ctx.Router.template.Lookup(name)
	if t == nil {
		return nil, fmt.Errorf("no such template '%s'", name)
	}
	return t, nil
}

func (r *Router) Redirect(w http.ResponseWriter, req *http.Request, url string, status ...int) {
	Redirect(w, req, url, status...)
}

func (r *Router) RedirectRoute(w http.ResponseWriter, req *http.Request, pathname string, status ...int) {
	var statusCode = http.StatusSeeOther
	if len(status) > 0 {
		statusCode = status[0]
	}

	// find the mathing route
	var handler http.Handler

	for _, route := range r.routes {
		// split prefix into method and path
		parts := strings.Split(route.prefix, " ")
		name := strings.TrimSpace(parts[1])
		if name == pathname {
			handler = route.handler
			break
		}
	}

	if handler == nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(statusCode)
	handler.ServeHTTP(w, req)
}

type routeInfo struct {
	Method string // Http method.
	Path   string // Registered pattern.
	Name   string // Function name for the handler.
}

func (r *Router) GetRegisteredRoutes() []routeInfo {
	var routes []routeInfo
	for _, route := range r.routes {
		parts := strings.SplitN(route.prefix, " ", 2)
		routes = append(routes, routeInfo{Method: parts[0], Path: parts[1], Name: getFuncName(route.handler)})
	}
	return routes
}

func getFuncName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

func ClientIPAddress(r *http.Request) (string, error) {
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")

	if len(splitIps) > 0 {
		// get last IP in list since ELB prepends other user defined IPs,
		// meaning the last one is the actual client IP.
		netIP := net.ParseIP(splitIps[len(splitIps)-1])
		if netIP != nil {
			return netIP.String(), nil
		}
	}

	// Try to get the IP from the X-Real-Ip header.
	ip := r.Header.Get("X-Real-Ip")
	if ip != "" {
		return ip, nil
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	netIP := net.ParseIP(ip)
	if netIP != nil {
		ip := netIP.String()
		if ip == "::1" {
			return "127.0.0.1", nil
		}
		return ip, nil
	}
	return "", errors.New("IP not found")
}
