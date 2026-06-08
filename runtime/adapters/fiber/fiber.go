// Package fiber adapts a generated GOWDK app handler to Fiber.
package fiber

import (
	"context"
	"net/http"

	fiberframework "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

type contextKey struct{}

// Context returns the Fiber context attached by this adapter.
func Context(ctx context.Context) (*fiberframework.Ctx, bool) {
	fiberContext, ok := ctx.Value(contextKey{}).(*fiberframework.Ctx)
	return fiberContext, ok
}

// Handler wraps the generated GOWDK http.Handler as a Fiber handler.
func Handler(handler http.Handler) fiberframework.Handler {
	return func(fiberContext *fiberframework.Ctx) error {
		wrapped := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			request = request.WithContext(context.WithValue(request.Context(), contextKey{}, fiberContext))
			handler.ServeHTTP(writer, request)
		})
		return adaptor.HTTPHandler(wrapped)(fiberContext)
	}
}

// Mount registers the generated GOWDK http.Handler on a Fiber app.
func Mount(router fiberframework.Router, pattern string, handler http.Handler) {
	router.All(pattern, Handler(handler))
}
