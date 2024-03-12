package egor

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func FormValue(req *http.Request, key string) string {
	return req.FormValue(key)
}

func FormData(req *http.Request) url.Values {
	err := req.ParseForm()
	if err != nil {
		panic(err)
	}
	return req.Form
}

// FormFile returns the first file from the multipart form with the given key.
func FormFile(req *http.Request, key string) (*multipart.FileHeader, error) {
	_, fh, err := req.FormFile(key)
	return fh, err
}

// FormFiles returns the files from the multipart form with the given key.
func FormFiles(req *http.Request, key string) ([]*multipart.FileHeader, error) {
	// get the file from multipart form
	fhs, ok := req.MultipartForm.File[key]
	if !ok {
		return nil, fmt.Errorf("file %s not found", key)
	}
	return fhs, nil
}

// ParseMultipartForm parses a request body as multipart form data.
func ParseMultipartForm(req *http.Request, maxMemory ...int64) (*multipart.Form, error) {
	var err error
	if len(maxMemory) > 0 {
		err = req.ParseMultipartForm(maxMemory[0])
	} else {
		err = req.ParseMultipartForm(req.ContentLength)
	}
	return req.MultipartForm, err
}

// BodyParser parses the request body and stores the result in v.
// v must be a pointer to a struct.
// Supported content types: application/json, application/x-www-form-urlencoded, multipart/form-data, application/xml
// For more robust form decoding we recommend using
// https://github.com/gorilla/schema package.
// Any form value can implement the FormScanner interface to implement custom form scanning.
func BodyParser(req *http.Request, v interface{}) error {
	// Make sure v is a pointer to a struct
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("v must be a pointer to a struct")
	}

	contentType := GetContentType(req)

	if contentType == ContentTypeJSON {
		decoder := json.NewDecoder(req.Body)
		return decoder.Decode(v)
	} else if contentType == ContentTypeXForm {
		err := req.ParseForm()
		if err != nil {
			return err
		}

		data := make(map[string]interface{})
		for k, v := range req.Form {
			if len(v) == 1 {
				data[k] = v[0] // if there's only one value.
			} else {
				data[k] = v // array of values or empty array
			}
		}
		return parseFormData(data, v)
	} else if contentType == ContentTypeMultipartForm {
		form, err := ParseMultipartForm(req)
		if err != nil {
			return err
		}

		data := make(map[string]interface{})
		for k, v := range form.Value {
			data[k] = v[0]
		}
		return parseFormData(data, v)
	} else if contentType == ContentTypeXML {
		xmlDecoder := xml.NewDecoder(req.Body)
		return xmlDecoder.Decode(v)
	} else {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func SnakeCase(s string) string {
	var res strings.Builder
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			res.WriteRune('_')
		}
		res.WriteRune(r)
	}
	return strings.ToLower(res.String())
}

// Parses the form data and stores the result in v.
// Default tag name is "form". You can specify a different tag name using the tag argument.
// Forexample "query" tag name will parse the form data using the "query" tag.
func parseFormData(data map[string]interface{}, v interface{}, tag ...string) error {
	var tagName string = "form"
	if len(tag) > 0 {
		tagName = tag[0]
	}

	rv := reflect.ValueOf(v).Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get(tagName)
		if tag == "" {
			tag = SnakeCase(field.Name)
		}

		tagList := strings.Split(tag, ",")
		for i := range tagList {
			tagList[i] = strings.TrimSpace(tagList[i])
		}

		// Take tag name to be the first in the tagList
		tag = tagList[0]

		required := field.Tag.Get("required") == "true"
		value, ok := data[tag]
		if !ok {
			if required {
				return fmt.Errorf("required field %s not found", tag)
			}
			continue
		}

		// set the value
		fieldVal := rv.Field(i)
		if err := setField(fieldVal, value); err != nil {
			return err
		}
	}

	return nil
}

func setField(fieldVal reflect.Value, value interface{}) error {
	// Dereference pointer if the field is a pointer
	if fieldVal.Kind() == reflect.Ptr {
		// Create a new value of the underlying type
		if fieldVal.IsNil() {
			fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
		}
		fieldVal = fieldVal.Elem()
	}

	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(value.(string))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(value.(string), 10, 64)
		if err != nil {
			return err
		}
		fieldVal.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(value.(string), 10, 64)
		if err != nil {
			return err
		}
		fieldVal.SetUint(v)
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(value.(string), 64)
		if err != nil {
			return err
		}
		fieldVal.SetFloat(v)
	case reflect.Bool:
		v, err := strconv.ParseBool(value.(string))
		if err != nil {
			// try parsing on/off since html forms use on/off for checkboxes
			if value.(string) == "on" {
				v = true
			} else if value.(string) == "off" {
				v = false
			} else {
				return err
			}
		}
		fieldVal.SetBool(v)
	case reflect.Slice:
		// Handle slice types
		return handleSlice(fieldVal, value)
	case reflect.Struct:
		if fieldVal.Type() == reflect.TypeOf(time.Time{}) {
			t, err := time.Parse(time.RFC3339, value.(string))
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(t))
		} else {
			// Check if the field implements the FormScanner interface
			if scanner, ok := fieldVal.Addr().Interface().(FormScanner); ok {
				return scanner.FormScan(value)
			}
			return fmt.Errorf("unsupported type: %s", fieldVal.Kind())
		}
	default:
		// check if the field implements the FormScanner interface (even if it's a pointer
		if fieldVal.Kind() == reflect.Ptr {
			if fieldVal.Elem().Kind() == reflect.Struct {
				if scanner, ok := fieldVal.Interface().(FormScanner); ok {
					return scanner.FormScan(value)
				}
			}
		} else if fieldVal.Kind() == reflect.Struct {
			// Check if the field implements the FormScanner interface
			if scanner, ok := fieldVal.Addr().Interface().(FormScanner); ok {
				return scanner.FormScan(value)
			}
		} else {
			return fmt.Errorf("unsupported type: %s", fieldVal.Kind())
		}
	}

	return nil
}

// Parses the form value and stores the result fieldVal.
// value should be a slice of strings.
func handleSlice(fieldVal reflect.Value, value any) error {

	valueSlice := value.([]string)
	sliceLen := len(valueSlice)
	if sliceLen == 0 {
		return nil
	}

	slice := reflect.MakeSlice(fieldVal.Type(), sliceLen, sliceLen)

	// get the kind of the slice element
	elemKind := fieldVal.Type().Elem().Kind()
	switch elemKind {
	case reflect.String:
		for i, v := range valueSlice {
			slice.Index(i).SetString(v)
		}
		fieldVal.Set(slice)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		for i, v := range valueSlice {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return err
			}
			slice.Index(i).SetInt(n)
		}
		fieldVal.Set(slice)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		for i, v := range valueSlice {
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return err
			}
			slice.Index(i).SetUint(n)
		}
		fieldVal.Set(slice)
	case reflect.Float32, reflect.Float64:
		for i, v := range valueSlice {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return err
			}
			slice.Index(i).SetFloat(n)
		}
		fieldVal.Set(slice)
	case reflect.Bool:
		for i, v := range valueSlice {
			n, err := strconv.ParseBool(v)
			if err != nil {
				// try parsing on/off since html forms use on/off for checkboxes
				if v == "on" {
					n = true
				} else if v == "off" {
					n = false
				} else {
					return err
				}
			}
			slice.Index(i).SetBool(n)
		}
		fieldVal.Set(slice)
	default:
		return fmt.Errorf("unsupported slice type: %s", elemKind)
	}
	return nil
}

// FormScanner is an interface for types that can scan form values.
// It is used to implement custom form scanning for types that are not supported by default.
type FormScanner interface {
	// FormScan scans the form value and stores the result in the receiver.
	FormScan(value interface{}) error
}

// QueryParser parses the query string and stores the result in v.
func QueryParser(req *http.Request, v interface{}, tag ...string) error {
	var tagName string = "query"
	if len(tag) > 0 {
		tagName = tag[0]
	}

	// Make sure v is a pointer to a struct
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("v must be a pointer to a struct")
	}

	data := req.URL.Query()
	dataMap := make(map[string]interface{}, len(data))
	for k, v := range data {
		if len(v) == 1 {
			dataMap[k] = v[0] // if there's only one value.
		} else {
			dataMap[k] = v // array of values or empty array
		}
	}

	return parseFormData(dataMap, v, tagName)
}
