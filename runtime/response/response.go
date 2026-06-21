package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/cssbruno/gowdk/runtime/validation"
)

// Kind identifies the response shape produced by actions, fragments, APIs, or
// full-page rendering.
type Kind string

const (
	HTML     Kind = "html"
	Redirect Kind = "redirect"
	Fragment Kind = "fragment"
	JSON     Kind = "json"
	Reload   Kind = "reload"
)

// SwapMode identifies how a fragment response should update its target.
type SwapMode string

const (
	SwapInnerHTML SwapMode = "innerHTML"
	SwapOuterHTML SwapMode = "outerHTML"
)

// Response is the generated runtime response envelope.
type Response struct {
	Kind    Kind
	Status  int
	Body    string
	Target  string
	Swap    SwapMode
	URL     string
	Cookies []http.Cookie
}

// HandlerError wraps failures raised by generated handlers.
type HandlerError struct {
	Status  int
	Message string
	Cause   error
}

func (err HandlerError) Error() string {
	if err.Message != "" {
		return err.Message
	}
	if err.Cause != nil {
		return err.Cause.Error()
	}
	if err.Status != 0 {
		return fmt.Sprintf("handler failed with status %d", err.Status)
	}
	return "handler failed"
}

func (err HandlerError) Unwrap() error {
	return err.Cause
}

// NewHandlerError creates an error suitable for generated handlers.
func NewHandlerError(status int, message string, cause error) error {
	return HandlerError{Status: status, Message: message, Cause: cause}
}

// HandlerStatus returns a handler error status, or fallback for ordinary errors.
func HandlerStatus(err error, fallback int) int {
	var handlerErr HandlerError
	if errors.As(err, &handlerErr) && handlerErr.Status != 0 {
		return handlerErr.Status
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}
	return fallback
}

// HandlerErrorMessage returns the client-facing message for a generated
// handler error. Ordinary 5xx failures use generic status text so internal
// details stay out of HTTP responses. HandlerError.Message remains an explicit
// application-owned override.
func HandlerErrorMessage(err error, status int) string {
	var handlerErr HandlerError
	if errors.As(err, &handlerErr) && strings.TrimSpace(handlerErr.Message) != "" {
		return handlerErr.Message
	}
	if status >= http.StatusInternalServerError {
		if text := http.StatusText(status); text != "" {
			return text
		}
		return http.StatusText(http.StatusInternalServerError)
	}
	if err != nil {
		return err.Error()
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return "request failed"
}

// WriteNoStoreHandlerError writes a generated handler error using the
// client-safe HandlerErrorMessage policy.
func WriteNoStoreHandlerError(writer http.ResponseWriter, err error, fallbackStatus int) {
	status := HandlerStatus(err, fallbackStatus)
	WriteNoStoreError(writer, status, HandlerErrorMessage(err, status))
}

// WriteNoStoreHandlerJSONError writes a generated handler error as the stable
// JSON shape used by contract web adapters.
func WriteNoStoreHandlerJSONError(writer http.ResponseWriter, err error, fallbackStatus int) {
	status := HandlerStatus(err, fallbackStatus)
	WriteNoStoreJSONError(writer, status, HandlerErrorMessage(err, status))
}

// HTMLBody creates a full HTML response.
func HTMLBody(status int, body string) Response {
	return Response{Kind: HTML, Status: status, Body: body}
}

// RedirectTo creates a redirect response.
func RedirectTo(url string) Response {
	return Response{Kind: Redirect, Status: 303, URL: url}
}

// ReloadPage asks enhanced clients to reload the current page. Non-enhanced
// forms should still use normal POST/redirect/get behavior.
func ReloadPage() Response {
	return Response{Kind: Reload, Status: http.StatusNoContent}
}

// WithCookie returns a copy of result that sets cookie when written to HTTP.
func WithCookie(result Response, cookie http.Cookie) Response {
	result.Cookies = append(result.Cookies, cookie)
	return result
}

// FragmentFor creates a partial fragment response for a DOM target.
func FragmentFor(target, body string) Response {
	return Response{Kind: Fragment, Status: 200, Target: target, Swap: SwapInnerHTML, Body: body}
}

// FragmentSwap creates a partial fragment response with an explicit swap mode.
func FragmentSwap(target string, swap SwapMode, body string) (Response, error) {
	if !validSwapMode(swap) {
		return Response{}, fmt.Errorf("unsupported fragment swap mode %q", swap)
	}
	return Response{Kind: Fragment, Status: 200, Target: target, Swap: swap, Body: body}, nil
}

// JSONBody creates a JSON response from an already-encoded body.
func JSONBody(status int, body string) Response {
	return Response{Kind: JSON, Status: status, Body: body}
}

// JSONValue marshals a value into a JSON response body.
func JSONValue(status int, value any) (Response, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return Response{}, err
	}
	return JSONBody(status, string(payload)), nil
}

// ValidationJSON creates a structured validation error response.
func ValidationJSON(result validation.Result) (Response, error) {
	return JSONValue(http.StatusUnprocessableEntity, struct {
		OK     bool               `json:"ok"`
		Errors []validation.Error `json:"errors"`
	}{OK: false, Errors: result.Errors})
}

// ValidationFragment creates a fragment response with escaped validation
// messages. It uses HTTP 200 so progressively enhanced partial forms can swap
// the returned fragment with the current client runtime.
func ValidationFragment(target string, result validation.Result) Response {
	return FragmentFor(target, ValidationHTML(result))
}

// ValidationHTML renders a small escaped validation message block.
func ValidationHTML(result validation.Result) string {
	parts := []string{`<div data-gowdk-validation role="alert" aria-live="polite">`}
	if len(result.Errors) > 0 {
		parts = append(parts, `<ul>`)
		for _, validationErr := range result.Errors {
			item := `<li`
			if validationErr.Field != "" {
				item += ` data-gowdk-field="` + html.EscapeString(validationErr.Field) + `"`
			}
			item += `>` + html.EscapeString(validationErr.Message) + `</li>`
			parts = append(parts, item)
		}
		parts = append(parts, `</ul>`)
	}
	parts = append(parts, `</div>`)
	return strings.Join(parts, "")
}

// WriteHTTP writes a runtime response envelope to net/http.
func WriteHTTP(writer http.ResponseWriter, result Response) error {
	status := statusOrDefault(result)
	if result.Kind == Redirect {
		if err := ValidateLocalRedirect(result.URL); err != nil {
			// Fail closed before writing any side-effect headers. This matches the
			// guard and SSR redirect lanes, which only allow local absolute paths.
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return err
		}
	}
	for _, cookie := range result.Cookies {
		http.SetCookie(writer, &cookie)
	}
	switch result.Kind {
	case Redirect:
		writer.Header().Set("Location", result.URL)
		writer.WriteHeader(status)
	case Reload:
		writer.Header().Set("X-GOWDK-Reload", "1")
		writer.WriteHeader(status)
	case Fragment:
		if statusAllowsBody(status) {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		if result.Target != "" {
			writer.Header().Set("X-GOWDK-Fragment-Target", result.Target)
		}
		if result.Swap != "" {
			writer.Header().Set("X-GOWDK-Fragment-Swap", string(result.Swap))
		}
		return writeBody(writer, status, result.Body)
	case JSON:
		if statusAllowsBody(status) {
			writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		return writeBody(writer, status, result.Body)
	default:
		if statusAllowsBody(status) {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		return writeBody(writer, status, result.Body)
	}
	return nil
}

// WriteNoStoreHTTP writes a response envelope that must not be cached.
func WriteNoStoreHTTP(writer http.ResponseWriter, result Response) error {
	writer.Header().Set("Cache-Control", "no-store")
	return WriteHTTP(writer, result)
}

// WriteNoStoreHTML writes a no-store HTML response and suppresses the body for
// HEAD requests.
func WriteNoStoreHTML(writer http.ResponseWriter, request *http.Request, body string) error {
	return WriteHTML(writer, request, body, "no-store")
}

// WriteHTML writes an HTML response with an explicit Cache-Control policy and
// suppresses the body for HEAD requests. An empty cache policy falls back to
// no-store for request-time safety.
func WriteHTML(writer http.ResponseWriter, request *http.Request, body string, cacheControl string) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	cacheControl = strings.TrimSpace(cacheControl)
	if existing := strings.TrimSpace(writer.Header().Get("Cache-Control")); strings.EqualFold(existing, "no-store") {
		cacheControl = existing
	}
	if cacheControl == "" {
		cacheControl = "no-store"
	}
	writer.Header().Set("Cache-Control", cacheControl)
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodHead {
		return nil
	}
	_, err := writer.Write([]byte(body))
	return err
}

// WriteNoStoreError writes an HTTP error that must not be cached.
func WriteNoStoreError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Cache-Control", "no-store")
	http.Error(writer, message, status)
}

// WriteNoStoreJSONError writes the stable generated JSON error shape.
func WriteNoStoreJSONError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Cache-Control", "no-store")
	result, err := JSONValue(status, struct {
		Error string `json:"error"`
	}{Error: message})
	if err != nil {
		WriteNoStoreError(writer, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	_ = WriteHTTP(writer, result)
}

func statusOrDefault(result Response) int {
	if result.Status != 0 {
		return result.Status
	}
	if result.Kind == Redirect {
		return http.StatusSeeOther
	}
	return http.StatusOK
}

func writeBody(writer http.ResponseWriter, status int, body string) error {
	writer.WriteHeader(status)
	if !statusAllowsBody(status) {
		return nil
	}
	_, err := writer.Write([]byte(body))
	return err
}

func statusAllowsBody(status int) bool {
	return status >= http.StatusOK && status != http.StatusNoContent && status != http.StatusNotModified
}

func validSwapMode(swap SwapMode) bool {
	switch swap {
	case SwapInnerHTML, SwapOuterHTML:
		return true
	default:
		return false
	}
}

// ValidateLocalRedirect reports whether url is a safe same-origin redirect
// target. Only local absolute paths are allowed; protocol-relative URLs,
// backslash tricks browsers normalize to "//", and CRLF header-injection
// attempts are rejected. It is the single redirect-safety contract shared by
// the response, guard, and SSR lanes so an attacker-influenced "next"/"return_to"
// value cannot become an open redirect.
func ValidateLocalRedirect(url string) error {
	if url == "" || url[0] != '/' {
		return fmt.Errorf("redirect %q must be a local absolute path", url)
	}
	if len(url) > 1 && (url[1] == '/' || url[1] == '\\') {
		return fmt.Errorf("redirect %q must not be protocol-relative", url)
	}
	// Browsers normalize "\" to "/" before navigating, so "/\evil.com" is
	// treated like the protocol-relative "//evil.com".
	if strings.Contains(url, "\\") {
		return fmt.Errorf("redirect %q must not contain backslashes", url)
	}
	if strings.ContainsAny(url, "\r\n") {
		return fmt.Errorf("redirect %q must not contain newlines", url)
	}
	return nil
}
