package actions

import "net/http"

// CSRFValidator validates action requests before generated handlers run.
type CSRFValidator interface {
	Validate(*http.Request) error
}

// NoopCSRF is useful for tests and local compiler-generated examples.
type NoopCSRF struct{}

// Validate accepts every request.
func (NoopCSRF) Validate(*http.Request) error {
	return nil
}
