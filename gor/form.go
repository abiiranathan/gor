package gor

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Parses form data and returns it to caller.
// Panics if req.ParseForm returns an error.
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
// You must call ParseMultipartForm first for req.MultipartForm to be populated.
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

// FormError represents an error encountered during body parsing.
type FormError struct {
	// The original error encountered.
	Err error
	// The kind of error encountered.
	Kind FormErrorKind
}

// FormErrorKind represents the kind of error encountered during body parsing.
type FormErrorKind string

const (
	// InvalidContentType indicates an unsupported content type.
	InvalidContentType FormErrorKind = "invalid_content_type"
	// InvalidStructPointer indicates that the provided v is not a pointer to a struct.
	InvalidStructPointer FormErrorKind = "invalid_struct_pointer"
	// RequiredFieldMissing indicates that a required field was not found.
	RequiredFieldMissing FormErrorKind = "required_field_missing"
	// UnsupportedType indicates that an unsupported type was encountered.
	UnsupportedType FormErrorKind = "unsupported_type"
	// ParseError indicates that an error occurred during parsing.
	ParseError FormErrorKind = "parse_error"
)

// Error implements the error interface.
func (e FormError) Error() string {
	return fmt.Sprintf("BodyParser error: kind=%s, err=%s", e.Kind, e.Err)
}

// BodyParser parses the request body and stores the result in v.
// v must be a pointer to a struct.
// Supported content types: application/json, application/x-www-form-urlencoded, multipart/form-data, application/xml
// For more robust form decoding we recommend using
// https://github.com/gorilla/schema package.
// Any form value can implement the FormScanner interface to implement custom form scanning.
// Struct tags are used to specify the form field name.
// If parsing forms, the default tag name is "form",
// followed by the "json" tag name, and then snake case of the field name.
func BodyParser(req *http.Request, v interface{}) error {
	// Make sure v is a pointer to a struct
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return FormError{
			Err:  fmt.Errorf("v must be a pointer to a struct"),
			Kind: InvalidStructPointer,
		}
	}

	contentType := GetContentType(req)

	if contentType == ContentTypeJSON {
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(v)
		if err != nil {
			return FormError{
				Err:  err,
				Kind: ParseError,
			}
		}
		return nil
	} else if contentType == ContentTypeUrlEncoded || contentType == ContentTypeMultipartForm {
		var form *multipart.Form
		var err error
		if contentType == ContentTypeMultipartForm {
			form, err = ParseMultipartForm(req)
			if err != nil {
				return FormError{
					Err:  err,
					Kind: ParseError,
				}
			}
		} else {
			err = req.ParseForm()
			if err != nil {
				return FormError{
					Err:  err,
					Kind: ParseError,
				}
			}
			form = &multipart.Form{
				Value: req.Form,
			}
		}

		data := make(map[string]interface{})

		for k, v := range form.Value {
			vLen := len(v)
			if vLen == 0 {
				continue // The struct will have the default value
			}

			if vLen == 1 {
				// skip empty values. Parsing "" to int, float, bool, etc causes errors.
				// Ignore to keep the default value of the struct field.
				// The user can check if the field is empty using the required tag.
				if v[0] == "" {
					continue
				}
				data[k] = v[0] // if there's only one value.
			} else {
				data[k] = v // array of values
			}
		}

		err = parseFormData(data, v)
		if err != nil {
			// propagate the error
			return err
		}
		return nil
	} else if contentType == ContentTypeXML {
		xmlDecoder := xml.NewDecoder(req.Body)
		err := xmlDecoder.Decode(v)
		if err != nil {
			return FormError{
				Err:  err,
				Kind: ParseError,
			}
		}
		return nil
	} else {
		return FormError{
			Err:  fmt.Errorf("unsupported content type: %s", contentType),
			Kind: InvalidContentType,
		}
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
			// try json tag name and fallback to snake case
			tag = field.Tag.Get("json")
			if tag == "" {
				tag = SnakeCase(field.Name)
			}
		}

		tagList := strings.Split(tag, ",")
		for i := range tagList {
			tagList[i] = strings.TrimSpace(tagList[i])
		}

		// Take tag name to be the first in the tagList
		tag = tagList[0]

		required := slices.Contains(tagList, "required") || field.Tag.Get("required") == "true"
		value, ok := data[tag]
		if !ok {
			if required {
				return FormError{
					Err:  fmt.Errorf("required field %s not found", tag),
					Kind: RequiredFieldMissing,
				}
			}
			continue
		}

		// set the value
		fieldVal := rv.Field(i)
		if err := setField(fieldVal, value); err != nil {
			return FormError{
				Err:  err,
				Kind: ParseError,
			}
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
			return FormError{
				Err:  fmt.Errorf("unsupported type: %s, a custom struct must implement gor.FormScanner interface", fieldVal.Kind()),
				Kind: UnsupportedType,
			}
		}
	default:
		// check if the field implements the FormScanner interface (even if it's a pointer)
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
			return FormError{
				Err:  fmt.Errorf("unsupported type: %s, a custom struct must implement gor.FormScanner interface", fieldVal.Kind()),
				Kind: UnsupportedType,
			}
		}
	}

	return nil
}

// Parses the form value and stores the result fieldVal.
// value should be a slice of strings.
func handleSlice(fieldVal reflect.Value, value any) error {
	var valueSlice []string
	var ok bool
	valueSlice, ok = value.([]string)
	if !ok {
		// Check if its a string and split it and clean it
		if v, ok := value.(string); ok {
			valueSlice = strings.Split(v, ",")
			for i := range valueSlice {
				valueSlice[i] = strings.TrimSpace(valueSlice[i])
			}
		} else {
			return FormError{
				Err:  fmt.Errorf("unsupported slice type: %T with value: %v", value, value),
				Kind: UnsupportedType,
			}
		}
	}

	sliceLen := len(valueSlice)
	if sliceLen == 0 {
		return nil // Use a zero value slice
	}

	// If we have a pointer to a slice, call handleSlice recursively
	if fieldVal.Kind() == reflect.Ptr {
		// We can't call of reflect.Value.Type on zero Value
		if fieldVal.IsNil() {
			fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
		}
		fieldVal = fieldVal.Elem()
		if fieldVal.Kind() == reflect.Slice {
			return handleSlice(fieldVal, valueSlice)
		}
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
	case reflect.Struct:
		// could be time.Time
		if fieldVal.Type().Elem() == reflect.TypeOf(time.Time{}) {
			for i, v := range valueSlice {
				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return err
				}
				slice.Index(i).Set(reflect.ValueOf(t))
			}
			fieldVal.Set(slice)
		} else {
			// Check if the slice element implements the FormScanner interface
			_, ok := reflect.New(fieldVal.Type().Elem()).Interface().(FormScanner)
			if !ok {
				return FormError{
					Err:  fmt.Errorf("unsupported slice element type: %s", fieldVal.Type().Elem().Kind()),
					Kind: UnsupportedType,
				}
			}

			for i, v := range valueSlice {
				// Create a new instance of the slice element
				elem := reflect.New(fieldVal.Type().Elem()).Elem()

				// Scan the form value into the slice element
				if err := setField(elem, v); err != nil {
					return err
				}

				// Set the element in the slice
				slice.Index(i).Set(elem)
			}

			fieldVal.Set(slice)
		}
	default:
		elemType := fieldVal.Type().Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}

		// Check if the slice element implements the FormScanner interface
		_, ok := reflect.New(elemType).Interface().(FormScanner)
		if !ok {
			return FormError{
				Err:  fmt.Errorf("unsupported slice element type: %s", elemType.Kind()),
				Kind: UnsupportedType,
			}
		}

		for i, v := range valueSlice {
			// Create a new instance of the slice element
			elem := reflect.New(elemType).Elem()

			// Scan the form value into the slice element
			if err := setField(elem, v); err != nil {
				return err
			}

			// Set the element in the slice
			slice.Index(i).Set(elem)
		}

		fieldVal.Set(slice)

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
		return FormError{
			Err:  fmt.Errorf("v must be a pointer to a struct"),
			Kind: InvalidStructPointer,
		}
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
