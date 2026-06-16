package actions

import "net/http"

// NoopCSRF is for package tests only.
type NoopCSRF struct{}

// Validate accepts every request.
func (NoopCSRF) Validate(*http.Request) error {
	return nil
}
