package ssr

import "net/http"

// Router registers generated SSR page handlers.
type Router interface {
	Handle(pattern string, handler http.Handler)
}

// Route is one generated SSR route binding.
type Route struct {
	Pattern string
	Handler http.Handler
}

// Register adds generated SSR routes to a router.
func Register(router Router, routes []Route) {
	for _, route := range routes {
		router.Handle(route.Pattern, route.Handler)
	}
}
