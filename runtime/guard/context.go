package guard

import (
	"context"
	"net/http"
)

// Context is passed to generated request-time guard functions.
type Context struct {
	Context context.Context
	Request *http.Request
	Session map[string]any
}

// NewContext creates the first-slice request context for generated guards.
func NewContext(request *http.Request, session map[string]any) Context {
	ctx := context.Background()
	if request != nil {
		ctx = request.Context()
	}
	return Context{
		Context: ctx,
		Request: request,
		Session: session,
	}
}
