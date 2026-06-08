// Package gin adapts a generated GOWDK app handler to Gin.
package gin

import (
	"context"
	"net/http"

	ginframework "github.com/gin-gonic/gin"
)

type contextKey struct{}

// Context returns the Gin context attached by this adapter.
func Context(ctx context.Context) (*ginframework.Context, bool) {
	ginContext, ok := ctx.Value(contextKey{}).(*ginframework.Context)
	return ginContext, ok
}

// Handler wraps the generated GOWDK http.Handler as a Gin handler.
func Handler(handler http.Handler) ginframework.HandlerFunc {
	return func(ginContext *ginframework.Context) {
		request := ginContext.Request.WithContext(context.WithValue(ginContext.Request.Context(), contextKey{}, ginContext))
		handler.ServeHTTP(ginContext.Writer, request)
	}
}

// Mount registers the generated GOWDK http.Handler on a Gin router.
func Mount(router ginframework.IRouter, pattern string, handler http.Handler) {
	router.Any(pattern, Handler(handler))
}
