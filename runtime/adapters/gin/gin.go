// Package gin adapts a generated GOWDK app handler to Gin.
package gin

import (
	"net/http"

	ginframework "github.com/gin-gonic/gin"
)

// Handler wraps the generated GOWDK http.Handler as a Gin handler.
func Handler(handler http.Handler) ginframework.HandlerFunc {
	return func(context *ginframework.Context) {
		handler.ServeHTTP(context.Writer, context.Request)
	}
}

// Mount registers the generated GOWDK http.Handler on a Gin router.
func Mount(router ginframework.IRouter, pattern string, handler http.Handler) {
	router.Any(pattern, Handler(handler))
}
