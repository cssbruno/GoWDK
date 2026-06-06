// Package echo adapts a generated GOWDK app handler to Echo.
package echo

import (
	"net/http"

	echoframework "github.com/labstack/echo/v5"
)

// Handler wraps the generated GOWDK http.Handler as an Echo handler.
func Handler(handler http.Handler) echoframework.HandlerFunc {
	return func(context *echoframework.Context) error {
		handler.ServeHTTP(context.Response(), context.Request())
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
