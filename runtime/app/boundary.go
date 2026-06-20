package app

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/cssbruno/gowdk/runtime/response"
)

// BoundaryLogger receives recovered panics from request-time handlers. The
// message and stack are secret-redacted before they reach it. Set it to nil to
// silence recovered-panic logging. It defaults to the standard log package.
var BoundaryLogger func(message string) = func(message string) {
	log.Print(message)
}

func logBoundaryPanic(kind string, value any) {
	logger := BoundaryLogger
	if logger == nil {
		return
	}
	detail := redactSecrets(fmt.Sprintf("%v", value))
	stack := redactSecrets(string(debug.Stack()))
	logger(fmt.Sprintf("gowdk: recovered panic in %s handler: %s\n%s", boundaryKindLabel(kind), detail, stack))
}

// Boundary wraps a generated request-time handler with a conservative panic
// boundary.
func Boundary(kind string, handler HandlerFunc) HandlerFunc {
	if handler == nil {
		return nil
	}
	kind = normalizeBoundaryKind(kind)
	return func(writer http.ResponseWriter, request *http.Request) (handled bool) {
		boundaryWriter := &boundaryResponseWriter{ResponseWriter: writer}
		wrappedWriter := wrapBoundaryResponseWriter(boundaryWriter)
		defer func() {
			if value := recover(); value != nil {
				if value == http.ErrAbortHandler {
					// Deliberate abort signal: let net/http kill the
					// connection without logging it as a failure.
					panic(value)
				}
				handled = true
				logBoundaryPanic(kind, value)
				if boundaryWriter.wrote {
					// The response already started; aborting the connection
					// is the only way to keep the client from treating the
					// truncated body as complete.
					panic(http.ErrAbortHandler)
				}
				writeBoundaryError(boundaryWriter, request, kind, value)
			}
		}()
		return handler(wrappedWriter, request)
	}
}

// BackendBoundary wraps a generated backend route handler.
func BackendBoundary(kind string, handler BackendHandler) BackendHandler {
	wrapped := Boundary(kind, HandlerFunc(handler))
	if wrapped == nil {
		return nil
	}
	return BackendHandler(wrapped)
}

type boundaryResponseWriter struct {
	http.ResponseWriter
	wrote bool
}

func (writer *boundaryResponseWriter) WriteHeader(status int) {
	writer.wrote = true
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *boundaryResponseWriter) Write(payload []byte) (int, error) {
	writer.wrote = true
	return writer.ResponseWriter.Write(payload)
}

func (writer *boundaryResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *boundaryResponseWriter) Written() bool {
	return writer.wrote
}

func normalizeBoundaryKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "action":
		return "action"
	case "api":
		return "api"
	case "command":
		return "command"
	case "query":
		return "query"
	case "ssr":
		return "ssr"
	default:
		return "backend"
	}
}

func writeBoundaryError(writer http.ResponseWriter, request *http.Request, kind string, value any) {
	_ = value
	message := fmt.Sprintf("GOWDK %s handler failed", boundaryKindLabel(kind))
	if kind == "ssr" && request != nil {
		WriteErrorPage(writer, request, http.StatusInternalServerError, message)
		return
	}
	if kind == "command" || kind == "query" {
		response.WriteNoStoreJSONError(writer, http.StatusInternalServerError, message)
		return
	}
	response.WriteNoStoreError(writer, http.StatusInternalServerError, message)
}

// RecoverSSRRoutePanic writes a no-store SSR route error page for a recovered
// generated route panic when the response has not started yet.
func RecoverSSRRoutePanic(writer http.ResponseWriter, request *http.Request, value any) {
	if value == nil {
		return
	}
	if value == http.ErrAbortHandler {
		panic(value)
	}
	logBoundaryPanic("ssr", value)
	if written, ok := writer.(interface{ Written() bool }); ok && written.Written() {
		panic(http.ErrAbortHandler)
	}
	WriteErrorPage(writer, request, http.StatusInternalServerError, "GOWDK SSR handler failed")
}

// RecoverEndpointPanic writes a no-store endpoint error page for a recovered
// generated action or API panic when the response has not started yet.
func RecoverEndpointPanic(writer http.ResponseWriter, request *http.Request, value any) {
	if value == nil {
		return
	}
	if value == http.ErrAbortHandler {
		panic(value)
	}
	logBoundaryPanic("endpoint", value)
	if written, ok := writer.(interface{ Written() bool }); ok && written.Written() {
		panic(http.ErrAbortHandler)
	}
	WriteErrorPage(writer, request, http.StatusInternalServerError, "GOWDK endpoint handler failed")
}

func boundaryKindLabel(kind string) string {
	switch kind {
	case "api":
		return "API"
	case "ssr":
		return "SSR"
	default:
		return kind
	}
}
