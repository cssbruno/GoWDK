package api

import (
	"net/http"
	"strings"

	"github.com/cssbruno/gowdk/runtime/response"
)

// ErrorBody is the default structured error payload for API helpers.
type ErrorBody struct {
	OK    bool      `json:"ok"`
	Error ErrorInfo `json:"error"`
}

// ErrorInfo describes one API error.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON creates a runtime response from a value encoded as JSON.
func JSON(status int, value any) (response.Response, error) {
	return response.JSONValue(status, value)
}

// Error creates a structured JSON API error response.
func Error(status int, code string, message string) (response.Response, error) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	code = strings.TrimSpace(code)
	if code == "" {
		code = "api_error"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = http.StatusText(status)
	}
	if message == "" {
		message = "request failed"
	}
	return JSON(status, ErrorBody{
		OK: false,
		Error: ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

// NoContent creates an empty successful API response.
func NoContent() response.Response {
	return response.Response{Kind: response.JSON, Status: http.StatusNoContent}
}
