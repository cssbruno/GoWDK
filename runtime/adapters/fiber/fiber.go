// Package fiber adapts a generated GOWDK app handler to Fiber.
package fiber

import (
	"net/http"

	fiberframework "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

// Handler wraps the generated GOWDK http.Handler as a Fiber handler.
func Handler(handler http.Handler) fiberframework.Handler {
	return adaptor.HTTPHandler(handler)
}

// Mount registers the generated GOWDK http.Handler on a Fiber app.
func Mount(app *fiberframework.App, pattern string, handler http.Handler) {
	app.All(pattern, Handler(handler))
}
