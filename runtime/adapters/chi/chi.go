// Package chi adapts a generated GOWDK app handler to chi.
package chi

import (
	"context"
	"net/http"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
	chiframework "github.com/go-chi/chi/v5"
)

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

// Context returns the chi route context attached by chi.
func Context(ctx context.Context) (*chiframework.Context, bool) {
	chiContext := chiframework.RouteContext(ctx)
	return chiContext, chiContext != nil
}

// Handler returns the generated GOWDK http.Handler for symmetry with other
// optional framework adapters.
func Handler(handler http.Handler) http.Handler {
	return handler
}

// Mount registers the generated GOWDK http.Handler at a catch-all chi mount
// point. Prefer MountRoutes or MountOpenAPI when route metadata is available.
func Mount(router chiframework.Router, pattern string, handler http.Handler) {
	router.Mount(pattern, handler)
}

// MountOpenAPI registers every routable method/path from a GOWDK OpenAPI report.
func MountOpenAPI(router chiframework.Router, spec []byte, handler http.Handler, options ...MountOption) error {
	routes, err := gowdkadapters.RoutesFromOpenAPI(spec)
	if err != nil {
		return err
	}
	if err := MountFallback(router, handler, options...); err != nil {
		return err
	}
	return MountRoutes(router, routes, handler, options...)
}

// MountFallback registers a catch-all route for generated apps whose OpenAPI
// report does not include page and asset paths.
func MountFallback(router chiframework.Router, handler http.Handler, options ...MountOption) error {
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	wrapped, err := gowdkadapters.HandlerWithPrefix(mount.prefix, handler)
	if err != nil {
		return err
	}
	pattern := "/"
	if mount.prefix != "" {
		pattern = mount.prefix
	}
	router.Mount(pattern, wrapped)
	return nil
}

// MountRoutes registers generated route metadata on a chi router.
func MountRoutes(router chiframework.Router, routes []gowdkadapters.Route, handler http.Handler, options ...MountOption) error {
	mount, err := resolveMountOptions(options)
	if err != nil {
		return err
	}
	wrapped, err := gowdkadapters.HandlerWithPrefix(mount.prefix, handler)
	if err != nil {
		return err
	}
	for _, route := range routes {
		pattern, err := gowdkadapters.TranslatePattern(route.Path, gowdkadapters.PatternChi)
		if err != nil {
			return err
		}
		pattern, err = gowdkadapters.JoinPrefix(mount.prefix, pattern)
		if err != nil {
			return err
		}
		router.Method(route.Method, pattern, wrapped)
	}
	return nil
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
