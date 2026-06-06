package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cssbruno/gowdk/runtime/response"
)

// Boundary wraps a generated request-time handler with a conservative panic
// boundary.
func Boundary(kind string, handler HandlerFunc) HandlerFunc {
	if handler == nil {
		return nil
	}
	kind = normalizeBoundaryKind(kind)
	return func(writer http.ResponseWriter, request *http.Request) (handled bool) {
		boundaryWriter := &boundaryResponseWriter{ResponseWriter: writer}
		defer func() {
			if value := recover(); value != nil {
				handled = true
				if !boundaryWriter.wrote {
					writeBoundaryError(boundaryWriter, request, kind, value)
				}
			}
		}()
		return handler(boundaryWriter, request)
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
	response.WriteNoStoreError(writer, http.StatusInternalServerError, message)
}

// RecoverSSRRoutePanic writes a no-store SSR route error page for a recovered
// generated route panic when the response has not started yet.
func RecoverSSRRoutePanic(writer http.ResponseWriter, request *http.Request, value any) {
	if value == nil {
		return
	}
	if written, ok := writer.(interface{ Written() bool }); ok && written.Written() {
		return
	}
	WriteErrorPage(writer, request, http.StatusInternalServerError, "GOWDK SSR handler failed")
}

// RecoverEndpointPanic writes a no-store endpoint error page for a recovered
// generated action or API panic when the response has not started yet.
func RecoverEndpointPanic(writer http.ResponseWriter, request *http.Request, value any) {
	if value == nil {
		return
	}
	if written, ok := writer.(interface{ Written() bool }); ok && written.Written() {
		return
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
