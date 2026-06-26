package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

var (
	// ErrNilRequest reports that a helper was called without an HTTP request.
	ErrNilRequest = errors.New("api request is nil")
	// ErrUnsupportedContentType reports a non-JSON request content type.
	ErrUnsupportedContentType = errors.New("unsupported API content type")
	// ErrMultipleJSONValues reports a JSON request body with trailing values.
	ErrMultipleJSONValues = errors.New("multiple JSON values in API request body")
)

// DecodeJSON decodes a JSON request body into T. Unknown object fields are
// rejected so handler inputs stay explicit.
func DecodeJSON[T any](request *http.Request) (T, error) {
	var value T
	if request == nil {
		return value, ErrNilRequest
	}
	if err := requireJSONContentType(request); err != nil {
		return value, err
	}
	if request.Body == nil {
		return value, fmt.Errorf("decode API JSON body: %w", io.EOF)
	}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode API JSON body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return value, ErrMultipleJSONValues
		}
		return value, fmt.Errorf("decode API JSON body: %w", err)
	}
	return value, nil
}

// RequireJSONContentType verifies that request declares a JSON media type.
func RequireJSONContentType(request *http.Request) error {
	if request == nil {
		return ErrNilRequest
	}
	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" {
		// Require an explicit JSON content type. A missing Content-Type is a
		// common cross-site request-forgery vector: a browser form post sends a
		// non-JSON default type, and accepting an empty type would let such a
		// request reach a JSON handler. Demanding application/json forces a
		// CORS preflight for cross-origin callers.
		return fmt.Errorf("%w: missing Content-Type", ErrUnsupportedContentType)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
	}
	mediaType = strings.ToLower(mediaType)
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedContentType, mediaType)
}

func requireJSONContentType(request *http.Request) error {
	return RequireJSONContentType(request)
}

// JSONFieldDecoder decodes one JSON object field at a time for generated API
// adapters. It accepts only a top-level object, preserves json.Number for
// integer parsing, and lets generated code reject unknown fields explicitly.
type JSONFieldDecoder struct {
	decoder *json.Decoder
}

// NewJSONFieldDecoder creates a field decoder for a strict JSON object body.
func NewJSONFieldDecoder(request *http.Request) (*JSONFieldDecoder, error) {
	if request == nil {
		return nil, ErrNilRequest
	}
	if err := RequireJSONContentType(request); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("decode API JSON body: %w", io.EOF)
	}
	decoder := json.NewDecoder(request.Body)
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode API JSON body: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("decode API JSON body: expected object")
	}
	return &JSONFieldDecoder{decoder: decoder}, nil
}

// More reports whether the current JSON object has another field.
func (decoder *JSONFieldDecoder) More() bool {
	return decoder != nil && decoder.decoder != nil && decoder.decoder.More()
}

// Field returns the next field name in the current JSON object.
func (decoder *JSONFieldDecoder) Field() (string, error) {
	if decoder == nil || decoder.decoder == nil {
		return "", ErrNilRequest
	}
	token, err := decoder.decoder.Token()
	if err != nil {
		return "", fmt.Errorf("decode API JSON field name: %w", err)
	}
	name, ok := token.(string)
	if !ok {
		return "", fmt.Errorf("decode API JSON field name: expected string")
	}
	return name, nil
}

// UnknownField returns the generated adapter error for an unsupported JSON
// object field. Generated adapters stop immediately, so no value is consumed.
func (decoder *JSONFieldDecoder) UnknownField(name string) error {
	return fmt.Errorf("decode API JSON body: unknown field %q", name)
}

// Finish consumes the object close token and rejects trailing JSON values.
func (decoder *JSONFieldDecoder) Finish() error {
	if decoder == nil || decoder.decoder == nil {
		return ErrNilRequest
	}
	token, err := decoder.decoder.Token()
	if err != nil {
		return fmt.Errorf("decode API JSON body: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '}' {
		return fmt.Errorf("decode API JSON body: expected object close")
	}
	extra, err := decoder.decoder.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode API JSON body: %w", err)
	}
	if extra != nil {
		return ErrMultipleJSONValues
	}
	return ErrMultipleJSONValues
}

// String decodes the next JSON field value as a string.
func (decoder *JSONFieldDecoder) String(name string) (string, error) {
	token, err := decoder.valueToken(name)
	if err != nil {
		return "", err
	}
	value, ok := token.(string)
	if !ok {
		return "", fmt.Errorf("decode API JSON field %q as string", name)
	}
	return value, nil
}

// Bool decodes the next JSON field value as a bool.
func (decoder *JSONFieldDecoder) Bool(name string) (bool, error) {
	token, err := decoder.valueToken(name)
	if err != nil {
		return false, err
	}
	value, ok := token.(bool)
	if !ok {
		return false, fmt.Errorf("decode API JSON field %q as bool", name)
	}
	return value, nil
}

// Int decodes the next JSON field value as a signed integer.
func (decoder *JSONFieldDecoder) Int(name string, bitSize int) (int64, error) {
	token, err := decoder.valueToken(name)
	if err != nil {
		return 0, err
	}
	number, ok := token.(json.Number)
	if !ok {
		return 0, fmt.Errorf("decode API JSON field %q as int", name)
	}
	parsed, err := strconv.ParseInt(number.String(), 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("decode API JSON field %q as int: %w", name, err)
	}
	return parsed, nil
}

// Uint decodes the next JSON field value as an unsigned integer.
func (decoder *JSONFieldDecoder) Uint(name string, bitSize int) (uint64, error) {
	token, err := decoder.valueToken(name)
	if err != nil {
		return 0, err
	}
	number, ok := token.(json.Number)
	if !ok {
		return 0, fmt.Errorf("decode API JSON field %q as uint", name)
	}
	parsed, err := strconv.ParseUint(number.String(), 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("decode API JSON field %q as uint: %w", name, err)
	}
	return parsed, nil
}

// Strings decodes the next JSON field value as an array of strings.
func (decoder *JSONFieldDecoder) Strings(name string) ([]string, error) {
	if decoder == nil || decoder.decoder == nil {
		return nil, ErrNilRequest
	}
	token, err := decoder.decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode API JSON field %q: %w", name, err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("decode API JSON field %q as []string", name)
	}
	var values []string
	for decoder.decoder.More() {
		token, err := decoder.decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("decode API JSON field %q: %w", name, err)
		}
		value, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("decode API JSON field %q as []string", name)
		}
		values = append(values, value)
	}
	token, err = decoder.decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode API JSON field %q: %w", name, err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != ']' {
		return nil, fmt.Errorf("decode API JSON field %q as []string", name)
	}
	return values, nil
}

func (decoder *JSONFieldDecoder) valueToken(name string) (json.Token, error) {
	if decoder == nil || decoder.decoder == nil {
		return nil, ErrNilRequest
	}
	token, err := decoder.decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode API JSON field %q: %w", name, err)
	}
	return token, nil
}

// QueryString returns the first query value for name.
func QueryString(request *http.Request, name string) (string, bool) {
	if request == nil || request.URL == nil {
		return "", false
	}
	values, ok := request.URL.Query()[name]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// QueryStrings returns all query values for name.
func QueryStrings(request *http.Request, name string) []string {
	if request == nil || request.URL == nil {
		return nil
	}
	values := request.URL.Query()[name]
	if len(values) == 0 {
		return nil
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}

// QueryBool returns the first query value for name as a bool.
func QueryBool(request *http.Request, name string) (bool, bool, error) {
	value, ok := QueryString(request, name)
	if !ok {
		return false, false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, true, fmt.Errorf("parse query %q as bool: %w", name, err)
	}
	return parsed, true, nil
}

// QueryInt returns the first query value for name as an int.
func QueryInt(request *http.Request, name string) (int, bool, error) {
	value, ok := QueryString(request, name)
	if !ok {
		return 0, false, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, true, fmt.Errorf("parse query %q as int: %w", name, err)
	}
	return parsed, true, nil
}

// QueryInt64 returns the first query value for name as an int64.
func QueryInt64(request *http.Request, name string) (int64, bool, error) {
	value, ok := QueryString(request, name)
	if !ok {
		return 0, false, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, true, fmt.Errorf("parse query %q as int64: %w", name, err)
	}
	return parsed, true, nil
}
