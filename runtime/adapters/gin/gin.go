// Package gin adapts a generated GOWDK app handler to Gin.
package gin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
	ginframework "github.com/gin-gonic/gin"
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

// MountOpenAPI registers every routable method/path from a GOWDK OpenAPI report.
func MountOpenAPI(router ginframework.IRouter, spec []byte, handler http.Handler, options ...MountOption) error {
	routes, err := gowdkadapters.RoutesFromOpenAPI(spec)
	if err != nil {
		return err
	}
	if len(routes) == 0 {
		return MountFallback(router, handler, options...)
	}
	if err := MountRoutes(router, routes, handler, options...); err != nil {
		return err
	}
	return MountNoRouteFallback(router, handler, options...)
}

// MountFallback registers catch-all routes for generated apps whose OpenAPI
// report does not include page and asset paths.
func MountFallback(router ginframework.IRouter, handler http.Handler, options ...MountOption) error {
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	wrapped, err := handlerWithPrefix(handler, mount.prefix)
	if err != nil {
		return err
	}
	for _, pattern := range ginFallbackPatterns(mount.prefix) {
		if err := registerGinFallback(router, pattern, wrapped); err != nil {
			return err
		}
	}
	return nil
}

// MountNoRouteFallback registers a generated handler fallback for Gin engines
// that already have route-specific registrations. Gin rejects catch-all routes
// beside concrete routes, so non-empty OpenAPI mounts use NoRoute instead.
func MountNoRouteFallback(router ginframework.IRouter, handler http.Handler, options ...MountOption) error {
	noRoute, ok := router.(interface {
		NoRoute(...ginframework.HandlerFunc)
	})
	if !ok {
		return fmt.Errorf("gin fallback mount with OpenAPI routes requires a router with NoRoute")
	}
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	wrapped, err := handlerWithPrefix(handler, mount.prefix)
	if err != nil {
		return err
	}
	noRoute.NoRoute(wrapped)
	return nil
}

// MountRoutes registers generated route metadata on a Gin router. Ambiguous Gin
// host patterns are returned as errors instead of panicking during registration.
func MountRoutes(router ginframework.IRouter, routes []gowdkadapters.Route, handler http.Handler, options ...MountOption) error {
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	wrapped, err := handlerWithPrefix(handler, mount.prefix)
	if err != nil {
		return err
	}
	registrations, err := ginRegistrations(routes, mount.prefix)
	if err != nil {
		return err
	}
	if conflict, ok := findGinConflict(registrations); ok {
		return fmt.Errorf("gin route conflict: %s %s conflicts with %s %s", conflict.Left.Method, conflict.Left.Route.Path, conflict.Right.Method, conflict.Right.Route.Path)
	}
	for _, registration := range registrations {
		if err := registerGinRoute(router, registration, wrapped); err != nil {
			return err
		}
	}
	return nil
}

func handlerWithPrefix(handler http.Handler, prefix string) (ginframework.HandlerFunc, error) {
	wrapped, err := gowdkadapters.HandlerWithPrefix(prefix, handler)
	if err != nil {
		return nil, err
	}
	return func(ginContext *ginframework.Context) {
		request := ginContext.Request.WithContext(context.WithValue(ginContext.Request.Context(), contextKey{}, ginContext))
		wrapped.ServeHTTP(ginContext.Writer, request)
	}, nil
}

func registerGinRoute(router ginframework.IRouter, registration ginRegistration, handler ginframework.HandlerFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("gin route conflict while mounting %s %s: %v", registration.Method, registration.Route.Path, recovered)
		}
	}()
	router.Handle(registration.Method, registration.HostPattern, handler)
	return nil
}

func registerGinFallback(router ginframework.IRouter, pattern string, handler ginframework.HandlerFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("gin fallback route conflict while mounting %s: %v", pattern, recovered)
		}
	}()
	router.Any(pattern, handler)
	return nil
}

type ginRegistration struct {
	Method      string
	HostPattern string
	Route       gowdkadapters.Route
}

type ginConflict struct {
	Left  ginRegistration
	Right ginRegistration
}

func ginRegistrations(routes []gowdkadapters.Route, prefix string) ([]ginRegistration, error) {
	registrations := make([]ginRegistration, 0, len(routes))
	for _, route := range routes {
		pattern, err := gowdkadapters.TranslatePattern(route.Path, gowdkadapters.PatternGin)
		if err != nil {
			return nil, err
		}
		pattern, err = gowdkadapters.JoinPrefix(prefix, pattern)
		if err != nil {
			return nil, err
		}
		registrations = append(registrations, ginRegistration{
			Method:      strings.ToUpper(strings.TrimSpace(route.Method)),
			HostPattern: pattern,
			Route:       route,
		})
	}
	return registrations, nil
}

func findGinConflict(registrations []ginRegistration) (ginConflict, bool) {
	for leftIndex := 0; leftIndex < len(registrations); leftIndex++ {
		for rightIndex := leftIndex + 1; rightIndex < len(registrations); rightIndex++ {
			left := registrations[leftIndex]
			right := registrations[rightIndex]
			if left.Method != right.Method {
				continue
			}
			if ginPatternsConflict(left.HostPattern, right.HostPattern) {
				return ginConflict{Left: left, Right: right}, true
			}
		}
	}
	return ginConflict{}, false
}

func ginPatternsConflict(left, right string) bool {
	leftSegments := splitPattern(left)
	rightSegments := splitPattern(right)
	for index := 0; ; index++ {
		leftDone := index >= len(leftSegments)
		rightDone := index >= len(rightSegments)
		if leftDone && rightDone {
			return true
		}
		if leftDone || rightDone {
			return false
		}
		leftKind := ginSegmentKind(leftSegments[index])
		rightKind := ginSegmentKind(rightSegments[index])
		if leftKind == "catchall" || rightKind == "catchall" {
			return true
		}
		if leftKind == "literal" && rightKind == "literal" && leftSegments[index] != rightSegments[index] {
			return false
		}
	}
}

func splitPattern(pattern string) []string {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func ginSegmentKind(segment string) string {
	if strings.HasPrefix(segment, "*") {
		return "catchall"
	}
	if strings.HasPrefix(segment, ":") {
		return "param"
	}
	return "literal"
}

func ginFallbackPatterns(prefix string) []string {
	if prefix == "" {
		return []string{"/*path"}
	}
	return []string{prefix, prefix + "/*path"}
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
