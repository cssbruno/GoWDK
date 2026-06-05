package spa

import "github.com/cssbruno/gowdk/runtime/response"

// PrerenderedPage is the build-time output for one SPA route.
type PrerenderedPage struct {
	Route string
	Path  string
	HTML  response.Response
}
