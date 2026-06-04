package ssr

import "net/http"

// ErrorHandler renders request-time SSR failures.
type ErrorHandler func(http.ResponseWriter, *http.Request, error)

// DefaultErrorHandler renders a conservative SSR failure response.
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	_ = r
	message := "GOWDK SSR error"
	if err != nil {
		message = "GOWDK SSR error: " + err.Error()
	}
	http.Error(w, message, http.StatusInternalServerError)
}
