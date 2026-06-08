// Package echo adapts a generated GOWDK app handler to Echo.
package echo

import (
	"context"
	"net/http"

	echoframework "github.com/labstack/echo/v5"
)

type contextKey struct{}

// Context returns the Echo context attached by this adapter.
func Context(ctx context.Context) (*echoframework.Context, bool) {
	echoContext, ok := ctx.Value(contextKey{}).(*echoframework.Context)
	return echoContext, ok
}

// Handler wraps the generated GOWDK http.Handler as an Echo handler.
func Handler(handler http.Handler) echoframework.HandlerFunc {
	return func(echoContext *echoframework.Context) error {
		request := echoContext.Request().WithContext(context.WithValue(echoContext.Request().Context(), contextKey{}, echoContext))
		handler.ServeHTTP(echoContext.Response(), request)
		return nil
	}
}

type router interface {
	Any(string, echoframework.HandlerFunc, ...echoframework.MiddlewareFunc) echoframework.RouteInfo
}

// Mount registers the generated GOWDK http.Handler on an Echo router.
func Mount(router router, pattern string, handler http.Handler) {
	router.Any(pattern, Handler(handler))
}
