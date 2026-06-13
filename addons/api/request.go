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

func requireJSONContentType(request *http.Request) error {
	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" {
		return nil
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
