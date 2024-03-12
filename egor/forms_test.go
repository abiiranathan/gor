package egor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type CustomStruct struct {
	Field1 string
	Field2 int
}

func (c *CustomStruct) FormScan(value interface{}) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not a string")
	}
	c.Field1 = v
	return nil
}

// Date in format YYYY-MM-DD
type Date time.Time

func (d *Date) FormScan(value interface{}) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not a string")
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

type customInt int // Kind is int

func TestSetField(t *testing.T) {
	tests := []struct {
		name      string
		fieldType reflect.Kind
		value     interface{}
		expected  interface{}
	}{
		{"String", reflect.String, "test", "test"},
		{"Int", reflect.Int, "123", 123},
		{"Uint", reflect.Uint, "123", uint(123)},
		{"Float32", reflect.Float32, "3.14", float32(3.14)},
		{"Bool True", reflect.Bool, "true", true},
		{"Bool True", reflect.Bool, "on", true},
		{"Bool True", reflect.Bool, "off", false},
		{"Bool False", reflect.Bool, "false", false},
		{"Slice", reflect.Slice, []string{"1", "2", "3"}, []string{"1", "2", "3"}},
		{"Time", reflect.Struct, "2022-02-22T12:00:00Z", time.Date(2022, 2, 22, 12, 0, 0, 0, time.UTC)},
		{"CustomStruct", reflect.Struct, "test", CustomStruct{Field1: "test"}},
		{"Date", reflect.Struct, "2022-02-22", Date(time.Date(2022, 2, 22, 0, 0, 0, 0, time.UTC))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fieldValue reflect.Value
			switch tt.fieldType {
			case reflect.String:
				fieldValue = reflect.ValueOf(new(string)).Elem()
			case reflect.Int:
				fieldValue = reflect.ValueOf(new(int)).Elem()
			case reflect.Uint:
				fieldValue = reflect.ValueOf(new(uint)).Elem()
			case reflect.Float32:
				fieldValue = reflect.ValueOf(new(float32)).Elem()
			case reflect.Bool:
				fieldValue = reflect.ValueOf(new(bool)).Elem()
			case reflect.Slice:
				fieldValue = reflect.ValueOf(new([]string)).Elem()
			case reflect.Struct:
				switch tt.name {
				case "Time":
					fieldValue = reflect.ValueOf(new(time.Time)).Elem()
				case "CustomStruct":
					fieldValue = reflect.ValueOf(&CustomStruct{}).Elem()
				case "Date":
					fieldValue = reflect.ValueOf(new(Date)).Elem()
				}
			}

			if err := setField(fieldValue, tt.value); err != nil {
				t.Errorf("setField() error = %v", err)
				return
			}

			if !reflect.DeepEqual(fieldValue.Interface(), tt.expected) {
				t.Errorf("setField() = %v, want %v", fieldValue.Interface(), tt.expected)
			}
		})
	}
}

func TestSetFieldCustomInt(t *testing.T) {
	fieldValue := reflect.ValueOf(new(customInt)).Elem()

	if err := setField(fieldValue, "123"); err != nil {
		t.Errorf("setField() error = %v", err)
		return
	}

	if !reflect.DeepEqual(fieldValue.Interface(), customInt(123)) {
		t.Errorf("setField() = %v, want %v", fieldValue.Interface(), customInt(123))
	}
}

// test pointers
func TestSetFieldsPointer(t *testing.T) {
	// use pointer to string, int, float, bool, slice, struct, time.Time, and custom type
	var (
		str   *string
		i     *int
		ui    *uint
		f32   *float32
		b     *bool
		slice *[]string
		c     *CustomStruct
		d     *Date
	)

	str = new(string)
	i = new(int)
	ui = new(uint)
	f32 = new(float32)
	b = new(bool)
	slice = new([]string)
	c = &CustomStruct{}
	d = new(Date)

	tests := []struct {
		name     string
		fieldPtr interface{}
		value    interface{}
		expected interface{}
	}{
		{"String", str, "test", "test"},
		{"Int", i, "123", 123},
		{"Uint", ui, "123", uint(123)},
		{"Float32", f32, "3.14", float32(3.14)},
		{"Bool True", b, "true", true},
		{"Bool True", b, "on", true},
		{"Bool True", b, "off", false},
		{"Bool False", b, "false", false},
		{"Slice", slice, []string{"1", "2", "3"}, []string{"1", "2", "3"}},
		{"CustomStruct", c, "test", CustomStruct{Field1: "test"}},
		{"Date", d, "2022-02-22", Date(time.Date(2022, 2, 22, 0, 0, 0, 0, time.UTC))},

		// test nil
		{"Nil", new(string), "test", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := setField(reflect.ValueOf(tt.fieldPtr).Elem(), tt.value); err != nil {
				t.Errorf("setField() error = %v", err)
				return
			}

			if !reflect.DeepEqual(reflect.ValueOf(tt.fieldPtr).Elem().Interface(), tt.expected) {
				t.Errorf("setField() = %v, want %v", reflect.ValueOf(tt.fieldPtr).Elem().Interface(), tt.expected)
			}
		})
	}
}

func TestSnakecase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty", "", ""},
		{"Lowercase", "test", "test"},
		{"Uppercase", "Test", "test"},
		{"CamelCase", "testString", "test_string"},
		{"CamelCase", "TestString", "test_string"},
		{"CamelCase", "testString123", "test_string123"},
		{"CamelCase", "TestString123", "test_string123"},
		{"CamelCase", "testString123Test", "test_string123_test"},
		{"CamelCase", "TestString123Test", "test_string123_test"},
		{"CamelCase", "testString123Test123", "test_string123_test123"},
		{"CamelCase", "TestString123Test123", "test_string123_test123"},
		{"CamelCase", "testString123Test123Test", "test_string123_test123_test"},
		{"CamelCase", "TestString123Test123Test", "test_string123_test123_test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SnakeCase(tt.input); got != tt.expected {
				t.Errorf("snakecase() = %v, want %v", got, tt.expected)
			}
		})
	}

}

// test multipart form with []int and []string
func TestSetFieldMultipartForm(t *testing.T) {
	type TestStruct struct {
		Ints    []int     `form:"ints"`
		Strings []string  `form:"strings"`
		Floats  []float64 `form:"floats"`
	}

	// send an actual form using httptest
	r := NewRouter()
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		var test TestStruct
		if err := BodyParser(r, &test); err != nil {
			t.Errorf("BodyParser() error = %v", err)
			return
		}

		if !reflect.DeepEqual(test.Ints, []int{1, 2, 3}) {
			t.Errorf("BodyParser() = %v, want %v", test.Ints, []int{1, 2, 3})
		}

		if !reflect.DeepEqual(test.Strings, []string{"a", "b", "c"}) {
			t.Errorf("BodyParser() = %v, want %v", test.Strings, []string{"a", "b", "c"})
		}

		if !reflect.DeepEqual(test.Floats, []float64{1.1, 2.2, 3.3}) {
			t.Errorf("BodyParser() = %v, want %v", test.Floats, []float64{1.1, 2.2, 3.3})
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// send a multipart form
	formData := url.Values{
		"ints":    {"1", "2", "3"},
		"strings": {"a", "b", "c"},
		"floats":  {"1.1", "2.2", "3.3"},
	}

	// Encode the form data
	formEncoded := formData.Encode()

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(formEncoded))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("BodyParser() status = %v, want %v", w.Code, http.StatusOK)
	}

}

// test application xml
func TestBodyParserXML(t *testing.T) {
	type TestStruct struct {
		Field1 string `xml:"field1"`
		Field2 int    `xml:"field2"`
	}

	r := NewRouter()
	r.Post("/submit", func(w http.ResponseWriter, r *http.Request) {
		var test TestStruct
		if err := BodyParser(r, &test); err != nil {
			t.Errorf("BodyParser() error = %v", err)
			return
		}

		if test.Field1 != "test" {
			t.Errorf("BodyParser() = %v, want %v", test.Field1, "test")
		}

		if test.Field2 != 123 {
			t.Errorf("BodyParser() = %v, want %v", test.Field2, 123)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// send an XML
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
	<testStruct>
		<field1>test</field1>
		<field2>123</field2>
	</testStruct>`

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(xmlData))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("BodyParser() status = %v, want %v", w.Code, http.StatusOK)
	}
}

// test QueryParser
func TestQueryParser(t *testing.T) {
	type TestStruct struct {
		Field1 string `query:"field1"`
		Field2 int    `query:"field2"`
	}

	r := NewRouter()
	r.Get("/submit", func(w http.ResponseWriter, r *http.Request) {
		var test TestStruct
		if err := QueryParser(r, &test); err != nil {
			t.Errorf("QueryParser() error = %v", err)
			return
		}

		if test.Field1 != "test" {
			t.Errorf("QueryParser() = %v, want %v", test.Field1, "test")
		}

		if test.Field2 != 123 {
			t.Errorf("QueryParser() = %v, want %v", test.Field2, 123)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// send a query
	req := httptest.NewRequest(http.MethodGet, "/submit?field1=test&field2=123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("QueryParser() status = %v, want %v", w.Code, http.StatusOK)
	}
}

// test query parser with slice
func TestQueryParserSlice(t *testing.T) {
	type TestStruct struct {
		Ints    []int     `query:"ints"`
		Strings []string  `query:"strings"`
		Floats  []float64 `query:"floats"`
	}

	r := NewRouter()
	r.Get("/submitslice", func(w http.ResponseWriter, r *http.Request) {
		var test TestStruct
		if err := QueryParser(r, &test); err != nil {
			t.Errorf("QueryParser() error = %v", err)
			return
		}

		if !reflect.DeepEqual(test.Ints, []int{1, 2, 3}) {
			t.Errorf("QueryParser() = %v, want %v", test.Ints, []int{1, 2, 3})
		}

		if !reflect.DeepEqual(test.Strings, []string{"a", "b", "c"}) {
			t.Errorf("QueryParser() = %v, want %v", test.Strings, []string{"a", "b", "c"})
		}

		if !reflect.DeepEqual(test.Floats, []float64{1.1, 2.2, 3.3}) {
			t.Errorf("QueryParser() = %v, want %v", test.Floats, []float64{1.1, 2.2, 3.3})
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// send a query
	req := httptest.NewRequest(http.MethodGet, "/submitslice?ints=1&ints=2&ints=3&strings=a&strings=b&strings=c&floats=1.1&floats=2.2&floats=3.3", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("QueryParser() status = %v, want %v", w.Code, http.StatusOK)
	}
}
