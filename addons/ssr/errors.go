package ssr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrorHandler renders request-time SSR failures.
type ErrorHandler func(http.ResponseWriter, *http.Request, error)

// DefaultErrorHandler renders a conservative SSR failure response.
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	_ = r
	message := "GOWDK SSR error"
	if err != nil {
		message = "GOWDK SSR error: " + err.Error()
	}
	http.Error(w, message, http.StatusInternalServerError)
}

// RedirectError asks generated SSR handlers to issue a safe local redirect.
type RedirectError struct {
	URL    string
	Status int
}

func (err RedirectError) Error() string {
	if err.URL == "" {
		return "SSR redirect"
	}
	return "SSR redirect to " + err.URL
}

// RedirectTo returns an error that generated SSR handlers translate into a
// no-store local redirect.
func RedirectTo(url string) error {
	return Redirect(url, http.StatusSeeOther)
}

// Redirect returns an error that generated SSR handlers translate into a
// no-store local redirect with the provided 3xx status.
func Redirect(url string, status int) error {
	if err := validateRedirectURL(url); err != nil {
		return err
	}
	if status < 300 || status > 399 {
		return fmt.Errorf("SSR redirect status must be 3xx")
	}
	return RedirectError{URL: url, Status: status}
}

// RedirectTarget extracts a generated SSR redirect error.
func RedirectTarget(err error) (string, int, bool) {
	var redirect RedirectError
	if !errors.As(err, &redirect) {
		return "", 0, false
	}
	status := redirect.Status
	if status == 0 {
		status = http.StatusSeeOther
	}
	return redirect.URL, status, true
}

func validateRedirectURL(url string) error {
	if !strings.HasPrefix(url, "/") {
		return fmt.Errorf("SSR redirect %q must be a local absolute path", url)
	}
	if strings.HasPrefix(url, "//") {
		return fmt.Errorf("SSR redirect %q must not be protocol-relative", url)
	}
	// Browsers normalize "\" to "/" before navigating, so "/\evil.com" is
	// treated like the protocol-relative "//evil.com".
	if strings.Contains(url, "\\") {
		return fmt.Errorf("SSR redirect %q must not contain backslashes", url)
	}
	if strings.ContainsAny(url, "\r\n") {
		return fmt.Errorf("SSR redirect %q must not contain newlines", url)
	}
	return nil
}
