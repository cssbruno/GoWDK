package ssr

import (
	"context"
	"net/http"
)

// LoadContext is passed to generated request-time load {} functions.
type LoadContext struct {
	Context context.Context
	Request *http.Request
	Session map[string]any
}

// LoadFunc is generated from a request-time load {} block.
type LoadFunc func(LoadContext) (map[string]any, error)

// NewLoadContext creates the first-slice request context for generated SSR load
// functions. Session storage is intentionally caller-supplied until the SSR
// addon defines secure session defaults.
func NewLoadContext(request *http.Request, session map[string]any) LoadContext {
	ctx := context.Background()
	if request != nil {
		ctx = request.Context()
	}
	return LoadContext{
		Context: ctx,
		Request: request,
		Session: session,
	}
}
