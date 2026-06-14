// Package echo adapts a generated GOWDK app handler to Echo.
package echo

import (
	"context"
	"fmt"
	"net/http"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
	echoframework "github.com/labstack/echo/v5"
)

type contextKey struct{}

type mountOptions struct {
	prefix string
}

// MountOption configures route-aware mounting.
type MountOption func(*mountOptions)

// WithPrefix mounts generated routes below a host-app prefix and strips that
// prefix before dispatching to the generated GOWDK handler.
func WithPrefix(prefix string) MountOption {
	return func(options *mountOptions) {
		options.prefix = prefix
	}
}

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

type fallbackRouter interface {
	Any(string, echoframework.HandlerFunc, ...echoframework.MiddlewareFunc) echoframework.RouteInfo
}

type routeRouter interface {
	Add(string, string, echoframework.HandlerFunc, ...echoframework.MiddlewareFunc) echoframework.RouteInfo
}

// Mount registers the generated GOWDK http.Handler on an Echo router.
func Mount(router fallbackRouter, pattern string, handler http.Handler) {
	router.Any(pattern, Handler(handler))
}

// MountOpenAPI registers every routable method/path from a GOWDK OpenAPI report.
func MountOpenAPI(router routeRouter, spec []byte, handler http.Handler, options ...MountOption) error {
	routes, err := gowdkadapters.RoutesFromOpenAPI(spec)
	if err != nil {
		return err
	}
	if err := MountFallback(router, handler, options...); err != nil {
		return err
	}
	return MountRoutes(router, routes, handler, options...)
}

// MountFallback registers catch-all routes for generated apps whose OpenAPI
// report does not include page and asset paths.
func MountFallback(router routeRouter, handler http.Handler, options ...MountOption) error {
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	anyRouter, ok := router.(fallbackRouter)
	if !ok {
		return fmt.Errorf("echo fallback mount requires a router with Any")
	}
	wrapped, err := handlerWithPrefix(handler, mount.prefix)
	if err != nil {
		return err
	}
	for _, pattern := range echoFallbackPatterns(mount.prefix) {
		anyRouter.Any(pattern, wrapped)
	}
	return nil
}

// MountRoutes registers generated route metadata on an Echo router.
func MountRoutes(router routeRouter, routes []gowdkadapters.Route, handler http.Handler, options ...MountOption) error {
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	wrapped, err := handlerWithPrefix(handler, mount.prefix)
	if err != nil {
		return err
	}
	for _, route := range routes {
		pattern, err := gowdkadapters.TranslatePattern(route.Path, gowdkadapters.PatternEcho)
		if err != nil {
			return err
		}
		pattern, err = gowdkadapters.JoinPrefix(mount.prefix, pattern)
		if err != nil {
			return err
		}
		router.Add(route.Method, pattern, wrapped)
	}
	return nil
}

func handlerWithPrefix(handler http.Handler, prefix string) (echoframework.HandlerFunc, error) {
	wrapped, err := gowdkadapters.HandlerWithPrefix(prefix, handler)
	if err != nil {
		return nil, err
	}
	return func(echoContext *echoframework.Context) error {
		request := echoContext.Request().WithContext(context.WithValue(echoContext.Request().Context(), contextKey{}, echoContext))
		wrapped.ServeHTTP(echoContext.Response(), request)
		return nil
	}, nil
}

func echoFallbackPatterns(prefix string) []string {
	if prefix == "" {
		return []string{"/*"}
	}
	return []string{prefix, prefix + "/*"}
}

func resolveMountOptions(options []MountOption) (mountOptions, error) {
	var mount mountOptions
	for _, option := range options {
		if option != nil {
			option(&mount)
		}
	}
	prefix, err := gowdkadapters.NormalizeMountPrefix(mount.prefix)
	if err != nil {
		return mountOptions{}, err
	}
	mount.prefix = prefix
	return mount, nil
}
