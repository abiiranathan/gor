package csrf

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"github.com/abiiranathan/gor/gor"
	"github.com/gorilla/sessions"
)

// Implement a CSRF middleware.
// This middleware checks for the presence of a CSRF token in the request.
// If the token is not present, or is invalid, it returns a 403 Forbidden.
// The token is expected to be in the request header, with the key "X-CSRF-Token"
// or in the request body, with the key "csrf_token".

const (
	// The default key to look for the CSRF token in the request header, query, form, or cookie.
	headerKeyName = "X-CSRF-Token"
	formKeyName   = "csrf_token"
	sessionName   = "csrf_session"
)

type TokenContextType string

var (
	ErrMissingHeader  = errors.New("missing CSRF token in request header")
	ErrMissingFormKey = errors.New("missing CSRF token in request body")
	ErrInvalidToken   = errors.New("invalid CSRF token")
	ErrMissingQuery   = errors.New("missing CSRF token in request query")
)

// Extract the CSRF token from the request header.
func FromHeader(req *http.Request, key string) (string, error) {
	token := req.Header.Get(key)
	if token == "" {
		return "", ErrMissingHeader
	}
	return token, nil
}

// Extract the CSRF token from the request body.
func FromForm(req *http.Request, key string) (string, error) {
	token := req.FormValue(key)
	if token == "" {
		return "", ErrMissingFormKey
	}
	return token, nil
}

// Extract the CSRF token from the request query.
func FromQuery(req *http.Request, key string) (string, error) {
	token := req.URL.Query().Get(key)
	if token == "" {
		return "", ErrMissingQuery
	}
	return token, nil
}

type csrf struct {
	// The key to look for the CSRF token in the request header, query, form, or cookie.
	// Defaults to "X-CSRF-Token".
	HeaderKeyName string

	// The key to look for the CSRF token in the request POST forms.
	// Defaults to "csrf_token".
	FormKeyName string

	// Name of the cookie session. defaults to "csrf_session"
	SessionName string

	// The function to call when the CSRF token is invalid.
	// If not set, the middleware will return a 403 Forbidden.
	// The function should write the response and return true if the request should continue.
	ErrorHandler func(w http.ResponseWriter, req *http.Request) bool

	// This store must implement the gorilla/sessions.Store interface.
	// If set, the middleware will store the CSRF token in the session.
	// The middleware will look for the CSRF token in the session first, before looking in the request.
	Store sessions.Store

	// Must satisfy the CSRFTokenGetter interface.
	// The function to call to get the CSRF token from the request.
	tokenGetter func(req *http.Request) (string, error)
}

// New returns a new CSRF middleware.
// Usage:
//
//	var store = sessions.NewCookieStore([]byte("secret key"))
//	store.Options = &sessions.Options{
//		Path:     "/",
//		MaxAge:   0,
//		Domain:   "localhost",
//		Secure:   false,
//		HttpOnly: true,
//		SameSite: http.SameSiteLaxMode,
//	}
//
//	mux.Use(middleware.New(store))
func New(store sessions.Store, options ...CSRFOption) gor.Middleware {
	c := &csrf{
		HeaderKeyName: headerKeyName,
		tokenGetter: func(req *http.Request) (string, error) {
			contentType := strings.Split(req.Header.Get("Content-Type"), ";")[0]

			switch contentType {
			case "application/x-www-form-urlencoded":
				return FromForm(req, formKeyName)
			case "multipart/form-data":
				return FromForm(req, formKeyName)
			case "application/json":
				return FromHeader(req, headerKeyName)
			default:
				return FromHeader(req, headerKeyName)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request) bool {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		},
		Store: store,
	}

	for _, opt := range options {
		opt(c)
	}

	return c.Middleware
}

type CSRFOption func(*csrf)

func WithHeaderKeyName(name string) CSRFOption {
	return func(c *csrf) {
		c.HeaderKeyName = name
	}
}

func WithFormKeyName(name string) CSRFOption {
	return func(c *csrf) {
		c.FormKeyName = name
	}
}

func WithSessionName(name string) CSRFOption {
	return func(c *csrf) {
		c.SessionName = name
	}
}

// Verify the CSRF token in the request against the token in the session.
func (c *csrf) verifyToken(req *http.Request) bool {
	session, err := c.Store.Get(req, sessionName)
	if err != nil {
		return false
	}

	expectedToken, ok := session.Values["token"].(string)
	if !ok {
		return false
	}

	token, err := c.tokenGetter(req)
	if err != nil {
		return false
	}

	return token == expectedToken
}

// createToken generates a random CSRF token.
func createToken() (string, error) {
	tokenBytes := make([]byte, 32) // Generate a 32-byte random token
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", err
	}
	token := base64.StdEncoding.EncodeToString(tokenBytes)
	escapedToken := template.HTMLEscapeString(token)
	return escapedToken, nil
}

// Middleware implements the CSRF protection middleware.
func (c *csrf) Middleware(next http.Handler) http.Handler {
	if c.Store == nil {
		panic("Store cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Get or create CSRF token.
		session, err := c.Store.Get(req, sessionName)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		token, ok := session.Values["token"].(string)
		if !ok || token == "" {
			token, err = createToken()
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			session.Values["token"] = token
			err = session.Save(req, w)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		}

		// Skip CSRF check for safe methods (GET, HEAD, OPTIONS, TRACE).
		if req.Method == http.MethodGet || req.Method == http.MethodHead ||
			req.Method == http.MethodOptions || req.Method == http.MethodTrace {
			// We still need to set the token in the response header for GET requests.
			// if the key is not valid, the next request will fail.
			w.Header().Set(c.HeaderKeyName, token)
			gor.SetContextValue(req, TokenContextType(formKeyName), token)

			// fmt.Println("Token:", token)
			next.ServeHTTP(w, req)
			return
		}

		// Verify CSRF token.
		if !c.verifyToken(req) {
			if c.ErrorHandler != nil && c.ErrorHandler(w, req) {
				return
			}
			http.Error(w, "CSRF token validation failed", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(req.Context(), TokenContextType(formKeyName), token)
		*req = *req.WithContext(ctx)

		// Continue with the next handler if all checks pass.
		next.ServeHTTP(w, req)
	})
}

func TokenFromRequest(req *http.Request) string {
	token, ok := gor.GetContextValue(req, TokenContextType(formKeyName)).(string)
	if !ok {
		return ""
	}
	return token
}
