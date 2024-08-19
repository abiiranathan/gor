package gor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSetAndGetContextValue(t *testing.T) {
	r := NewRouter()

	r.Get("/context", func(w http.ResponseWriter, req *http.Request) {
		SetContextValue(req, "key", "value")

		if GetContextValue(req, "key") != "value" {
			t.Error("SetContextValue() failed")
		}
	})

	req := httptest.NewRequest("GET", "/context", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

}

func TestSendJSON(t *testing.T) {
	r := NewRouter()

	data := map[string]string{"key": "value"}
	r.Get("/sendjson", func(w http.ResponseWriter, req *http.Request) {
		SendJSON(w, data)
	})

	req := httptest.NewRequest("GET", "/sendjson", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// unmarschal the response
	// check if the response is the same as the data
	expected := "{\"key\":\"value\"}\n" // json.Encoder appends a newline

	// check content type
	if w.Header().Get("Content-Type") != ContentTypeJSON {
		t.Errorf("Content-Type is not application/json")
	}

	if w.Body.String() != expected {
		t.Errorf("SendJSON() failed, expected %s, got %q", expected, w.Body.String())
	}
}

func TestSendString(t *testing.T) {
	r := NewRouter()

	r.Get("/sendstring", func(w http.ResponseWriter, req *http.Request) {
		SendString(w, "Hello")
	})

	req := httptest.NewRequest("GET", "/sendstring", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// check content type
	if w.Header().Get("Content-Type") != ContentTypeText {
		t.Errorf("Content-Type is not text/plain")
	}

	if w.Body.String() != "Hello" {
		t.Errorf("SendString() failed, expected Hello, got %q", w.Body.String())
	}
}

func TestSendHTML(t *testing.T) {
	r := NewRouter()

	r.Get("/sendhtml", func(w http.ResponseWriter, req *http.Request) {
		SendHTML(w, "<html>hello</html>")
	})

	req := httptest.NewRequest("GET", "/sendhtml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// check content type
	if w.Header().Get("Content-Type") != ContentTypeHTML {
		t.Errorf("Content-Type is not text/html")
	}

	if w.Body.String() != "<html>hello</html>" {
		t.Errorf("SendHTML() failed, expected <html>hello</html>, got %q", w.Body.String())
	}
}

func TestSendFile(t *testing.T) {
	r := NewRouter()

	r.Get("/sendfile", func(w http.ResponseWriter, req *http.Request) {
		SendFile(w, req, "request_test.go")
	})

	req := httptest.NewRequest("GET", "/sendfile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// check content type
	if w.Header().Get("Content-Type") != "text/x-go; charset=utf-8" {
		t.Errorf("Content-Type is not correct: %s", w.Header().Get("Content-Type"))
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(w.Body)

	if buf.String() == "" {
		t.Errorf("SendFile() failed, expected file contents, got %q", w.Body.String())
	}

	// check length
	if w.Header().Get("Content-Length") == "" {
		t.Errorf("Content-Length is empty")
	}

	// make sure the first line is a package declaration
	if buf.String()[:8] != "package " {
		t.Errorf("SendFile() failed, expected package declaration, got %q", buf.String()[:8])
	}
}

func TestSendError(t *testing.T) {
	r := NewRouter()

	r.Get("/testsenderror", func(w http.ResponseWriter, req *http.Request) {
		SendError(w, req, errors.New("Not Found"), http.StatusNotFound)
	})

	req := httptest.NewRequest("GET", "/testsenderror", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "Not Found" {
		t.Errorf("SendError() failed, expected Not Found, got %q", w.Body.String())
	}
}

func TestSendJSONError(t *testing.T) {
	r := NewRouter()

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		SendJSONError(w, map[string]any{"key": "value"}, http.StatusBadRequest)
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := "{\"key\":\"value\"}\n"
	if w.Body.String() != expected {
		t.Errorf("expected %q, got %q", expected, w.Body.String())
	}
}

func TestSendJSONError2(t *testing.T) {
	r := NewRouter()
	r.Get("/jsonerror", func(w http.ResponseWriter, r *http.Request) {
		SendJSONError(w, map[string]any{"error": "Something went wrong"}, http.StatusBadRequest)
	})

	req := httptest.NewRequest("GET", "/jsonerror", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %q, got %q", http.StatusBadRequest, w.Code)
	}

	res := make(map[string]any)
	json.Unmarshal(w.Body.Bytes(), &res)

	errStr, ok := res["error"]
	if !ok {
		t.Errorf("expected map to contain error key")
	}

	expected := "Something went wrong"
	if errStr != expected {
		t.Errorf("Expected map key to be %v, got %v", expected, errStr)
	}

}

func TestGetContentType(t *testing.T) {
	testCases := []struct {
		contentType string
		expected    string
	}{
		{"text/html; charset=utf-8", "text/html"},
		{"multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW", "multipart/form-data"},
		{"application/json", "application/json"},
	}

	for _, tc := range testCases {
		t.Run(tc.contentType, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/getcontent", nil)
			req.Header.Set("Content-Type", tc.contentType)

			actual := GetContentType(req)
			if actual != tc.expected {
				t.Errorf("GetContentType() failed, expected %q, got %q", tc.expected, actual)
			}
		})
	}

}

func TestRedirect(t *testing.T) {
	r := NewRouter()

	r.Get("/startpage", func(w http.ResponseWriter, req *http.Request) {
		Redirect(w, req, "/redirected", http.StatusFound)
	})

	req := httptest.NewRequest("GET", "/startpage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Redirect() failed, expected status code %d, got %d", http.StatusFound, w.Code)
	}

}

func TestQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/query?key=value", nil)

	actual := Query(req, "key")
	if actual != "value" {
		t.Errorf("Query() failed, expected value, got %q", actual)
	}

	// test default value
	actual = Query(req, "key2", "default")
	if actual != "default" {
		t.Errorf("Query() failed, expected default, got %q", actual)
	}
}

func TestQueryInt(t *testing.T) {
	req := httptest.NewRequest("GET", "/query?key=1", nil)

	actual := QueryInt(req, "key")
	if actual != 1 {
		t.Errorf("QueryInt() failed, expected 1, got %d", actual)
	}

	// test default value
	actual = QueryInt(req, "key2", 10)
	if actual != 10 {
		t.Errorf("QueryInt() failed, expected 10, got %d", actual)
	}
}

func TestParamInt(t *testing.T) {
	req := httptest.NewRequest("GET", "/param/1", nil)
	req.SetPathValue("key", "1") // we have to set the path value manually b'se we are not using the http.ServeMux

	actual := ParamInt(req, "key")
	if actual != 1 {
		t.Errorf("ParamInt() failed, expected 1, got %d", actual)
	}

	// test default value
	actual = ParamInt(req, "key2", 10)
	if actual != 10 {
		t.Errorf("ParamInt() failed, expected 10, got %d", actual)
	}
}

func TestSaveFile(t *testing.T) {
	// create temp file
	f, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	dst, err := os.CreateTemp("", "destination")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dst.Name())

	// close the file
	f.Close()

	// create a file header
	r := NewRouter()
	r.Post("/uploadfile", func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(r.ContentLength)
		_, file, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}

		err = SaveFile(file, dst.Name())
		if err != nil {
			t.Fatal(err)
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Hello World!")
	})

	// send the form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", f.Name())
	if err != nil {
		t.Error(err)
	}

	_, err = part.Write([]byte("hello world"))
	if err != nil {
		t.Error(err)
	}

	// close writer before creating request
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/uploadfile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	data, err := io.ReadAll(w.Body)
	if err != nil {
		t.Error(err)
	}

	if string(data) != "Hello World!" {
		t.Errorf("expected Hello World!, got %s", string(data))
	}

}
