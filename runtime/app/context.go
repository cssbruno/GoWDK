package app

import (
	"context"
	"net/http"
)

type contextKey string

const (
	requestContextKey  contextKey = "gowdk-request"
	paramsContextKey   contextKey = "gowdk-params"
	csrfContextKey     contextKey = "gowdk-csrf"
	sessionContextKey  contextKey = "gowdk-session"
	routeContextKey    contextKey = "gowdk-route"
	endpointContextKey contextKey = "gowdk-endpoint"
)

// RouteMetadata describes one generated request-time page route.
type RouteMetadata struct {
	Kind          string
	PageID        string
	Method        string
	Path          string
	Render        string
	DynamicParams []string
	Guards        []string
	HasLoad       bool
}

// EndpointMetadata describes one generated backend endpoint declaration.
type EndpointMetadata struct {
	Kind   string
	PageID string
	Name   string
	Method string
	Path   string
}

// WithRequest stores the current HTTP request in a context for generated
// backend handlers.
func WithRequest(ctx context.Context, request *http.Request) context.Context {
	return context.WithValue(ctx, requestContextKey, request)
}

// Request returns the HTTP request attached by generated runtime adapters.
func Request(ctx context.Context) (*http.Request, bool) {
	request, ok := ctx.Value(requestContextKey).(*http.Request)
	return request, ok
}

// WithParams stores route params in a context.
func WithParams(ctx context.Context, params map[string]string) context.Context {
	copied := map[string]string{}
	for key, value := range params {
		copied[key] = value
	}
	return context.WithValue(ctx, paramsContextKey, copied)
}

// Params returns a copy of route params attached by generated runtime adapters.
func Params(ctx context.Context) map[string]string {
	params, _ := ctx.Value(paramsContextKey).(map[string]string)
	copied := map[string]string{}
	for key, value := range params {
		copied[key] = value
	}
	return copied
}

// WithCSRF stores a generated CSRF token in a context.
func WithCSRF(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfContextKey, token)
}

// CSRF returns the generated CSRF token attached to a request context.
func CSRF(ctx context.Context) string {
	token, _ := ctx.Value(csrfContextKey).(string)
	return token
}

// WithSession stores application session state in a context.
func WithSession(ctx context.Context, session any) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// Session returns application session state attached to a request context.
func Session(ctx context.Context) any {
	return ctx.Value(sessionContextKey)
}

// WithRoute stores generated route metadata in a context.
func WithRoute(ctx context.Context, route RouteMetadata) context.Context {
	route.DynamicParams = copyStrings(route.DynamicParams)
	route.Guards = copyStrings(route.Guards)
	return context.WithValue(ctx, routeContextKey, route)
}

// Route returns generated route metadata attached by generated runtime
// adapters.
func Route(ctx context.Context) (RouteMetadata, bool) {
	route, ok := ctx.Value(routeContextKey).(RouteMetadata)
	route.DynamicParams = copyStrings(route.DynamicParams)
	route.Guards = copyStrings(route.Guards)
	return route, ok
}

// WithEndpoint stores generated endpoint metadata in a context.
func WithEndpoint(ctx context.Context, endpoint EndpointMetadata) context.Context {
	return context.WithValue(ctx, endpointContextKey, endpoint)
}

// Endpoint returns generated endpoint metadata attached by generated runtime
// adapters.
func Endpoint(ctx context.Context) (EndpointMetadata, bool) {
	endpoint, ok := ctx.Value(endpointContextKey).(EndpointMetadata)
	return endpoint, ok
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}
