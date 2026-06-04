package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Kind identifies the response shape produced by actions, fragments, APIs, or
// full-page rendering.
type Kind string

const (
	HTML     Kind = "html"
	Redirect Kind = "redirect"
	Fragment Kind = "fragment"
	JSON     Kind = "json"
)

// SwapMode identifies how a fragment response should update its target.
type SwapMode string

const (
	SwapInnerHTML SwapMode = "innerHTML"
	SwapOuterHTML SwapMode = "outerHTML"
)

// Response is the generated runtime response envelope.
type Response struct {
	Kind   Kind
	Status int
	Body   string
	Target string
	Swap   SwapMode
	URL    string
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
	return fallback
}

// HTMLBody creates a full HTML response.
func HTMLBody(status int, body string) Response {
	return Response{Kind: HTML, Status: status, Body: body}
}

// RedirectTo creates a redirect response.
func RedirectTo(url string) Response {
	return Response{Kind: Redirect, Status: 303, URL: url}
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

// WriteHTTP writes a runtime response envelope to net/http.
func WriteHTTP(writer http.ResponseWriter, result Response) error {
	status := statusOrDefault(result)
	switch result.Kind {
	case Redirect:
		writer.Header().Set("Location", result.URL)
		writer.WriteHeader(status)
	case Fragment:
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if result.Target != "" {
			writer.Header().Set("X-GOWDK-Fragment-Target", result.Target)
		}
		if result.Swap != "" {
			writer.Header().Set("X-GOWDK-Fragment-Swap", string(result.Swap))
		}
		writer.WriteHeader(status)
		_, err := writer.Write([]byte(result.Body))
		return err
	case JSON:
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(status)
		_, err := writer.Write([]byte(result.Body))
		return err
	default:
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(status)
		_, err := writer.Write([]byte(result.Body))
		return err
	}
	return nil
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

func validSwapMode(swap SwapMode) bool {
	switch swap {
	case SwapInnerHTML, SwapOuterHTML:
		return true
	default:
		return false
	}
}
