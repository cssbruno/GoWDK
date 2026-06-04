package embed

import "net/http"

// Assets exposes embedded frontend artifacts through http.FileSystem.
type Assets struct {
	FS http.FileSystem
}

// FileServer returns an HTTP handler for embedded assets.
func (assets Assets) FileServer() http.Handler {
	return http.FileServer(assets.FS)
}
