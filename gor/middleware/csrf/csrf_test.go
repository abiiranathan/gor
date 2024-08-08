package csrf_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abiiranathan/gor/gor"
	"github.com/abiiranathan/gor/gor/middleware/csrf"
	"github.com/gorilla/sessions"
)

// test csrf.go

type user struct {
	Name string
	Age  int
}

func TestCSRF(t *testing.T) {
	router := gor.NewRouter()

	store := sessions.NewCookieStore([]byte("super secret token"))
	router.Use(csrf.New(store))

	router.Get("/csrf", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello CSRF"))
	})

	router.Post("/csrf", func(w http.ResponseWriter, r *http.Request) {
		var u user
		err := gor.BodyParser(r, &u)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		gor.SendJSON(w, u)
	})

	// create request
	req := httptest.NewRequest("GET", "/csrf", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// check if the response is 200, we/GET /csrf should not be blocked
	if w.Code != 200 {
		t.Errorf("GET /csrf failed: %d", w.Code)
	}

	token := w.Header().Get("X-CSRF-Token")

	// create request
	u := user{Name: "John Doe", Age: 25}

	b, _ := json.Marshal(u)
	body := bytes.NewReader(b)

	req = httptest.NewRequest("POST", "/csrf", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", token)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// TODO: Fix this test
	// // check if the response is 200, we/POST /csrf should not be blocked
	// if w.Code != 200 {
	// 	t.Errorf("POST /csrf failed: %d", w.Code)
	// }

	// // create request
	// req = httptest.NewRequest("POST", "/csrf", nil)
	// w = httptest.NewRecorder()
	// router.ServeHTTP(w, req)

	// // check if the response is 403, we/POST /csrf should be blocked
	// if w.Code != 403 {
	// 	t.Errorf("POST /csrf failed: %d", w.Code)
	// }
}
