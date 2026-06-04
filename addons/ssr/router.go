package ssr

import "net/http"

// Router registers generated SSR page handlers.
type Router interface {
	Handle(pattern string, handler http.Handler)
}
