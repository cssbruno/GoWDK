package guard

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cssbruno/gowdk/runtime/response"
)

// RedirectError asks generated guard handling to issue a safe local redirect.
type RedirectError struct {
	URL    string
	Status int
}

func (err RedirectError) Error() string {
	if err.URL == "" {
		return "guard redirect"
	}
	return "guard redirect to " + err.URL
}

// ResponseError asks generated guard handling to write a runtime response.
type ResponseError struct {
	Result response.Response
}

func (err ResponseError) Error() string {
	return "guard response"
}

// RedirectTo returns an error that generated guard handling translates into a
// no-store local redirect.
func RedirectTo(url string) error {
	return Redirect(url, http.StatusSeeOther)
}

// Redirect returns an error that generated guard handling translates into a
// no-store local redirect with the provided 3xx status.
func Redirect(url string, status int) error {
	if err := validateRedirectURL(url); err != nil {
		return err
	}
	if status < 300 || status > 399 {
		return fmt.Errorf("guard redirect status must be 3xx")
	}
	return RedirectError{URL: url, Status: status}
}

// Respond returns an error that generated guard handling translates into a
// no-store runtime response.
func Respond(result response.Response) error {
	return ResponseError{Result: result}
}

// ResponseResult extracts a generated guard response error.
func ResponseResult(err error) (response.Response, bool) {
	var redirect RedirectError
	if errors.As(err, &redirect) {
		return redirectResponseResult(redirect)
	}
	var redirectPtr *RedirectError
	if errors.As(err, &redirectPtr) && redirectPtr != nil {
		return redirectResponseResult(*redirectPtr)
	}
	var result ResponseError
	if errors.As(err, &result) {
		return result.Result, true
	}
	var resultPtr *ResponseError
	if errors.As(err, &resultPtr) && resultPtr != nil {
		return resultPtr.Result, true
	}
	return response.Response{}, false
}

func redirectResponseResult(redirect RedirectError) (response.Response, bool) {
	status := redirect.Status
	if status == 0 {
		status = http.StatusSeeOther
	}
	if status < 300 || status > 399 {
		return response.Response{}, false
	}
	if validateRedirectURL(redirect.URL) != nil {
		return response.Response{}, false
	}
	return response.Response{Kind: response.Redirect, Status: status, URL: redirect.URL}, true
}

// WriteNoStoreFailure writes a generated guard failure response. Ordinary
// guard errors fail closed with 403; guard response helpers keep the same
// no-store cache policy while allowing explicit redirects or response shapes.
func WriteNoStoreFailure(writer http.ResponseWriter, err error) {
	if result, ok := ResponseResult(err); ok {
		_ = response.WriteNoStoreHTTP(writer, result)
		return
	}
	response.WriteNoStoreError(writer, http.StatusForbidden, err.Error())
}

func validateRedirectURL(url string) error {
	if url == "" || url[0] != '/' {
		return fmt.Errorf("guard redirect %q must be a local absolute path", url)
	}
	if len(url) > 1 && (url[1] == '/' || url[1] == '\\') {
		return fmt.Errorf("guard redirect %q must not be protocol-relative", url)
	}
	// Browsers normalize "\" to "/" before navigating, so "/\evil.com" is
	// treated like the protocol-relative "//evil.com".
	if strings.Contains(url, "\\") {
		return fmt.Errorf("guard redirect %q must not contain backslashes", url)
	}
	if strings.ContainsAny(url, "\r\n") {
		return fmt.Errorf("guard redirect %q must not contain newlines", url)
	}
	return nil
}
