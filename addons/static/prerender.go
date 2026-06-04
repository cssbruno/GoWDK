package static

import "github.com/cssbruno/gowdk/runtime/response"

// PrerenderedPage is the build-time output for one static route.
type PrerenderedPage struct {
	Route string
	Path  string
	HTML  response.Response
}
