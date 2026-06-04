package ssr

import "net/http"

// ErrorHandler renders request-time SSR failures.
type ErrorHandler func(http.ResponseWriter, *http.Request, error)
